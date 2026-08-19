package pwlsp

// The api:cli-lsp server loop.
//
// It serves the parse-only half of requirement:pw-language-server: syntax
// diagnostics and a document outline, for open documents only. It reads no
// file it was not sent, loads no project, and writes nothing, which is what
// policy:editor-tool-execution requires of a process an editor starts on open.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
	"sync"
)

// Options are what api:cli-lsp resolved from its arguments.
type Options struct {
	// Root is the workspace root the client named, or the one --root forced.
	// Nothing reads it yet; it is carried so the project loading of
	// requirement:pw-language-server has one place to arrive.
	Root string
	// Log receives protocol tracing. Nil is off, which is the default: a
	// server logging by default writes a file into a workspace it promised not
	// to write to.
	Log io.Writer
	// Name and Version identify this server to the client.
	Name    string
	Version string
	// Load resolves the project model. It is injected because the
	// data:project-config reader lives with the CLI, and a second reader here
	// would answer the same question a second way. Nil serves the parse-only
	// mode requirement:pw-language-server describes for a file with no project.
	Load Loader
	// HintFamilies switches the requirement:editor-inlay-hints families. Nil
	// takes the defaults, which is what a client that configures nothing gets.
	HintFamilies map[BindingKind]bool
}

// document is one open buffer. The client owns the text from didOpen until
// didClose, so nothing here reads the file from disk: the buffer and the file
// differ exactly when the developer is typing, which is when diagnostics
// matter most.
type document struct {
	uri     string
	kind    Dialect
	version int
	text    string
	starts  lineStarts
	found   analysis
}

// Server holds the open documents and the lifecycle state one connection has.
type Server struct {
	options Options
	writer  io.Writer
	// mutex guards documents and the writer. Requests are handled one at a
	// time, so it exists for the writer's benefit rather than for concurrency
	// this server does not yet have.
	mutex        sync.Mutex
	documents    map[string]*document
	initialized  bool
	shuttingDown bool
	// project holds the resolved half. Its own lock is separate from the one
	// above because a reload walks the tree, and holding the document lock for
	// that would stall every keystroke.
	project projectState
	// loaded marks that loading was attempted, so the parse-only mode is
	// entered once rather than retried on every document.
	loaded bool
	// configDiagnosticURI is the configuration file a load error was reported
	// against, kept so the finding is cleared when the next load succeeds.
	configDiagnosticURI string
}

// NewServer returns a server that writes its messages to writer.
func NewServer(writer io.Writer, options Options) *Server {
	if options.Name == "" {
		options.Name = "pw"
	}
	if options.HintFamilies == nil {
		options.HintFamilies = defaultHintFamilies()
	}
	return &Server{options: options, writer: writer, documents: map[string]*document{}}
}

// Serve reads messages until the stream ends or exit is received. It returns
// the exit code api:cli-lsp reports: zero after a shutdown request, and
// nonzero when the connection ended without one, which is what tells a client
// its restart policy applies.
func (s *Server) Serve(reader io.Reader) int {
	buffered := bufio.NewReader(reader)
	for {
		body, err := readMessage(buffered)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return s.exitCode()
			}
			s.trace("read: %v", err)
			return 1
		}
		var message incoming
		if err := json.Unmarshal(body, &message); err != nil {
			s.respondError(nil, codeParseError, "the message is not JSON: "+err.Error())
			continue
		}
		if message.Method == "exit" {
			return s.exitCode()
		}
		s.handle(message)
	}
}

// exitCode follows the protocol: a server that was asked to shut down exits
// zero, and one whose stream ended without a shutdown exits nonzero.
func (s *Server) exitCode() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.shuttingDown {
		return 0
	}
	return 1
}

func (s *Server) handle(message incoming) {
	s.trace("<- %s", message.Method)
	if message.isRequest() {
		s.handleRequest(message)
		return
	}
	s.handleNotification(message)
}

func (s *Server) handleRequest(message incoming) {
	switch message.Method {
	case "initialize":
		s.respond(message.ID, s.initialize(message.Params))
	case "shutdown":
		s.mutex.Lock()
		s.shuttingDown = true
		s.mutex.Unlock()
		s.respond(message.ID, nil)
	case "textDocument/documentSymbol":
		s.symbolRequest(message)
	case "workspace/symbol":
		s.workspaceSymbolRequest(message)
	case "textDocument/hover":
		s.hoverRequest(message)
	case "textDocument/definition":
		s.definitionRequest(message)
	case "textDocument/references":
		s.referencesRequest(message)
	case "textDocument/completion":
		s.completionRequest(message)
	case "textDocument/inlayHint":
		s.inlayHintRequest(message)
	// Two requests of this server's own, for the surfaces LSP has no method
	// for. They are namespaced so a client cannot mistake one for a standard
	// capability, and a client that does not know them never sends one.
	case "pw/generatedFor":
		s.generatedForRequest(message)
	case "pw/routes":
		s.routesRequest(message)
	case "pw/storyFor":
		s.storyForRequest(message)
	case "pw/project":
		project, _, _ := s.project.snapshot()
		s.respond(message.ID, projectInfo(project))
	default:
		s.respondError(message.ID, codeMethodNotFound, "pw lsp does not serve "+message.Method)
	}
}

func (s *Server) handleNotification(message incoming) {
	switch message.Method {
	case "initialized":
		// The project is loaded here rather than inside initialize, so a
		// client is not kept waiting on a tree walk before it has the
		// capabilities it asked for.
		s.ensureProject()
	case "workspace/didChangeWatchedFiles":
		var params didChangeWatchedFilesParams
		if s.decode(message, &params) && params.namesProjectConfig() {
			// requirement:pw-language-server: a popcornweb.toml change reloads
			// the model without restarting the process.
			s.reloadProject()
		}
	case "textDocument/didOpen":
		var params didOpenParams
		if s.decode(message, &params) {
			s.openDocument(params.TextDocument)
		}
	case "textDocument/didChange":
		var params didChangeParams
		if s.decode(message, &params) {
			s.changeDocument(params)
		}
	case "textDocument/didSave":
		// The buffer is already current; a save changes nothing this server
		// has read. It is accepted rather than refused so a client may
		// register for it without an error.
	case "textDocument/didClose":
		var params didCloseParams
		if s.decode(message, &params) {
			s.closeDocument(params.TextDocument.URI)
		}
	default:
		s.trace("ignoring notification %s", message.Method)
	}
}

// decode reads a notification's parameters, tracing rather than answering when
// they are unusable: a notification has no reply to put an error in.
func (s *Server) decode(message incoming, target any) bool {
	if err := json.Unmarshal(message.Params, target); err != nil {
		s.trace("%s: unusable params: %v", message.Method, err)
		return false
	}
	return true
}

func (s *Server) initialize(raw json.RawMessage) initializeResult {
	var params initializeParams
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &params)
	}
	s.mutex.Lock()
	if s.options.Root == "" {
		s.options.Root = params.workspaceRoot()
	}
	s.initialized = true
	s.mutex.Unlock()

	return initializeResult{
		Capabilities: serverCapabilities{
			TextDocumentSync: textDocumentSyncOptions{
				OpenClose: true,
				// Full sync: the documents here are one screen of template and
				// the parsers take a whole buffer anyway, so incremental sync
				// would add a patch path with nothing to gain from it.
				Change: syncFull,
				Save:   &saveOptions{IncludeText: false},
			},
			DocumentSymbolProvider:  true,
			WorkspaceSymbolProvider: true,
			HoverProvider:           true,
			DefinitionProvider:      true,
			ReferencesProvider:      true,
			InlayHintProvider:       true,
			CompletionProvider:      &completionOptions{TriggerCharacters: []string{"<", "{"}},
		},
		ServerInfo: serverInfo{Name: s.options.Name, Version: s.options.Version},
	}
}

func (s *Server) openDocument(item textDocumentItem) {
	kind := dialectOf(item.URI)
	if kind == dialectNone {
		s.trace("didOpen: %s is not a template source", item.URI)
		return
	}
	s.mutex.Lock()
	doc := &document{uri: item.URI, kind: kind, version: item.Version}
	s.documents[item.URI] = doc
	s.mutex.Unlock()
	// A document can arrive before the client sends initialized, and it also
	// supplies a place to look for the project when no root was named.
	s.ensureProject()
	s.setText(doc, item.Text, item.Version)
}

func (s *Server) changeDocument(params didChangeParams) {
	s.mutex.Lock()
	doc := s.documents[params.TextDocument.URI]
	s.mutex.Unlock()
	if doc == nil {
		s.trace("didChange for an unopened document %s", params.TextDocument.URI)
		return
	}
	if len(params.ContentChanges) == 0 {
		return
	}
	// Full sync, so the last change is the whole document.
	s.setText(doc, params.ContentChanges[len(params.ContentChanges)-1].Text, params.TextDocument.Version)
}

func (s *Server) closeDocument(uri string) {
	s.mutex.Lock()
	_, open := s.documents[uri]
	delete(s.documents, uri)
	s.mutex.Unlock()
	if !open {
		return
	}
	// A closed document keeps no diagnostics: the client owns the buffer, and
	// what this server reported about it is no longer about anything it holds.
	s.publish(uri, nil, []Diagnostic{})
}

// setText replaces a document's content and republishes what the parse found.
func (s *Server) setText(doc *document, text string, version int) {
	s.mutex.Lock()
	doc.text = text
	doc.version = version
	doc.starts = newLineStarts(text)
	doc.found = analyze(fileNameOf(doc.uri), doc.kind, text, doc.starts)
	diagnostics := doc.found.diagnostics
	s.mutex.Unlock()
	s.publish(doc.uri, &version, append(diagnostics, s.projectDiagnostics(doc)...))
}

// projectDiagnostics are the findings that need the model rather than the
// parse. There is one today: a source no generation purpose compiles, reported
// in the words api:cli-generate uses, per decision:shared-check-catalog.
//
// It is a warning rather than an error because the file is valid; what is
// wrong is that nothing reads it, which api:cli-generate also reports without
// stopping.
func (s *Server) projectDiagnostics(doc *document) []Diagnostic {
	project, _, _ := s.project.snapshot()
	if project == nil {
		return nil
	}
	path := filePathOf(doc.uri)
	if path == doc.uri {
		return nil
	}
	message, stray := project.strayMessage(path)
	if !stray {
		return nil
	}
	return []Diagnostic{{
		// The whole first line: the finding is about the file rather than
		// about anything written in it, and marking one token would suggest
		// the token is the problem.
		Range:    doc.starts.rangeAt(doc.text, 0),
		Severity: severityWarning,
		Source:   "pw",
		Message:  message,
	}}
}

func (s *Server) symbolRequest(message incoming) {
	var params documentSymbolParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		s.respondError(message.ID, codeInvalidParams, "unusable params: "+err.Error())
		return
	}
	s.mutex.Lock()
	doc := s.documents[params.TextDocument.URI]
	s.mutex.Unlock()
	if doc == nil {
		s.respondError(message.ID, codeRequestFailed, "no open document at "+params.TextDocument.URI)
		return
	}
	s.mutex.Lock()
	symbols := documentSymbols(doc.found, doc.text, doc.starts)
	s.mutex.Unlock()
	s.respond(message.ID, symbols)
}

func (s *Server) publish(uri string, version *int, diagnostics []Diagnostic) {
	if diagnostics == nil {
		diagnostics = []Diagnostic{}
	}
	s.send(notification{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params: publishDiagnosticsParams{
			URI:         uri,
			Version:     version,
			Diagnostics: diagnostics,
		},
	})
}

func (s *Server) respond(id json.RawMessage, value any) {
	s.send(result{JSONRPC: "2.0", ID: id, Result: value})
}

func (s *Server) respondError(id json.RawMessage, code int, message string) {
	if id == nil {
		id = json.RawMessage("null")
	}
	s.send(failure{JSONRPC: "2.0", ID: id, Error: responseError{Code: code, Message: message}})
}

func (s *Server) send(message any) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if err := writeMessage(s.writer, message); err != nil {
		s.traceLocked("write: %v", err)
	}
}

func (s *Server) trace(format string, args ...any) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.traceLocked(format, args...)
}

func (s *Server) traceLocked(format string, args ...any) {
	if s.options.Log == nil {
		return
	}
	fmt.Fprintf(s.options.Log, format+"\n", args...)
}

// ensureProject loads the model once, on whichever of the initialized
// notification and the first document arrives first. A client that never sends
// initialized still gets resolved answers, and a client that does is not
// charged for the walk twice.
func (s *Server) ensureProject() {
	s.mutex.Lock()
	if s.loaded {
		s.mutex.Unlock()
		return
	}
	s.loaded = true
	s.mutex.Unlock()
	s.loadProject()
}

// reloadProject rebuilds the model after a configuration change.
func (s *Server) reloadProject() {
	s.mutex.Lock()
	s.loaded = true
	s.mutex.Unlock()
	s.loadProject()
	// Every open document is republished: a document whose diagnostics were
	// answered under the old model may answer differently under the new one,
	// and a finding nobody cleared would outlive the configuration that
	// produced it.
	for _, doc := range s.openDocuments() {
		// The project half of a document's findings changes with the model,
		// so the republish assembles both halves rather than resending the
		// parse alone.
		s.publish(doc.uri, &doc.version, append(doc.found.diagnostics, s.projectDiagnostics(doc)...))
	}
}

// loadProject resolves the model and reports what it found. The three outcomes
// are the ones api:cli-lsp names: a project, no project, and a project that
// will not load.
func (s *Server) loadProject() {
	if s.options.Load == nil {
		s.project.replace(nil, index{}, nil)
		return
	}
	s.mutex.Lock()
	start := s.options.Root
	s.mutex.Unlock()
	if start == "" {
		start = s.anyOpenDirectory()
	}
	if start == "" {
		// Nothing names a place to look from yet. The next document opened
		// supplies one, so this is not the parse-only decision.
		s.mutex.Lock()
		s.loaded = false
		s.mutex.Unlock()
		return
	}

	project, err := s.options.Load(start)
	switch {
	case errors.Is(err, ErrNoProject):
		s.project.replace(nil, index{}, nil)
		s.announceOnce(
			"No popcornweb.toml above " + start + ". Syntax diagnostics work; " +
				"anything needing the project reports unavailable rather than guessing.",
		)
		s.clearConfigDiagnostics()
	case err != nil:
		// An unreadable project keeps syntax analysis running. The error is a
		// diagnostic on the file that holds it rather than a log line, because
		// the developer's next move is to open that file and fix it.
		s.project.replace(nil, index{}, err)
		s.publishConfigError(start, err)
	default:
		built := buildIndex(project)
		s.project.replace(project, built, nil)
		s.trace("project %s: %d declarations in %d sources", project.Root, len(built.declarations), built.files)
		s.clearConfigDiagnostics()
	}
}

// anyOpenDirectory is the directory of some open document, used to find the
// project when the client named no root at all.
func (s *Server) anyOpenDirectory() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, doc := range s.documents {
		if path := filePathOf(doc.uri); path != doc.uri {
			return filepath.Dir(path)
		}
	}
	return ""
}

func (s *Server) openDocuments() []*document {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	open := make([]*document, 0, len(s.documents))
	for _, doc := range s.documents {
		open = append(open, doc)
	}
	return open
}

// configDiagnosticURI is the file a load error is reported against. It is
// remembered so the finding can be cleared when the next load succeeds.
func (s *Server) publishConfigError(start string, err error) {
	uri := uriOf(filepath.Join(start, "popcornweb.toml"))
	s.mutex.Lock()
	s.configDiagnosticURI = uri
	s.mutex.Unlock()
	s.publish(uri, nil, []Diagnostic{{
		Range:    Range{},
		Severity: severityError,
		Source:   "pw",
		Message:  "the project did not load: " + err.Error(),
	}})
}

func (s *Server) clearConfigDiagnostics() {
	s.mutex.Lock()
	uri := s.configDiagnosticURI
	s.configDiagnosticURI = ""
	s.mutex.Unlock()
	if uri != "" {
		s.publish(uri, nil, []Diagnostic{})
	}
}

// announceOnce tells the client something about the session's own state. It is
// window/logMessage rather than a popup: a checkout with no project is a normal
// thing to open, and interrupting for it would be wrong.
func (s *Server) announceOnce(message string) {
	s.project.mutex.Lock()
	if s.project.reported {
		s.project.mutex.Unlock()
		return
	}
	s.project.reported = true
	s.project.mutex.Unlock()

	s.send(notification{
		JSONRPC: "2.0",
		Method:  "window/logMessage",
		Params:  logMessageParams{Type: logInfo, Message: message},
	})
}

func (s *Server) workspaceSymbolRequest(message incoming) {
	var params workspaceSymbolParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		s.respondError(message.ID, codeInvalidParams, "unusable params: "+err.Error())
		return
	}
	project, built, _ := s.project.snapshot()
	if project == nil {
		// No project means no scope to search. An answer assembled from the
		// open documents alone would look like a workspace search and cover
		// whatever happened to be open, which is the guessing the parse-only
		// mode exists to refuse.
		s.respond(message.ID, []SymbolInformation{})
		return
	}
	s.respond(message.ID, workspaceSymbols(params.Query, built, s.openSymbols(project)))
}

// openSymbols is what the editor's own buffers contribute, which is what keeps
// an answer ahead of the file on disk.
//
// A document no purpose lists contributes nothing, per
// requirement:editor-workspace-symbols: the search covers the scope
// api:cli-generate reads, and a .pw.html outside every declared purpose is
// absent from that scope whether or not it happens to be open.
func (s *Server) openSymbols(project *Project) []openSymbols {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	open := make([]openSymbols, 0, len(s.documents))
	for _, doc := range s.documents {
		path := filePathOf(doc.uri)
		if _, owned := project.owns(path, doc.kind); !owned {
			continue
		}
		open = append(open, openSymbols{
			uri:       doc.uri,
			container: filepath.ToSlash(relativeTo(project.Root, path)),
			symbols:   documentSymbols(doc.found, doc.text, doc.starts),
		})
	}
	return open
}

// resolveAt finds the declaration the caret names, in the graph as it stands
// with the open buffers overlaid.
//
// It returns the word too, so a caller can say what it failed to resolve
// rather than answering an unexplained nothing.
func (s *Server) resolveAt(params textDocumentPositionParams) (Symbol, string, Range, bool) {
	s.mutex.Lock()
	doc := s.documents[params.TextDocument.URI]
	s.mutex.Unlock()
	if doc == nil {
		return Symbol{}, "", Range{}, false
	}
	s.mutex.Lock()
	word, at := wordAt(doc.text, doc.starts, params.Position)
	s.mutex.Unlock()
	if word == "" {
		return Symbol{}, "", Range{}, false
	}

	graph := s.currentGraph()
	if graph == nil {
		return Symbol{}, word, at, false
	}
	symbol, resolved := graph.Resolve(params.TextDocument.URI, word)
	return symbol, word, at, resolved
}

// currentGraph is the indexed graph with every open buffer's declarations
// replacing the indexed copy of that file, so a name declared and not yet
// saved resolves the same as one on disk.
func (s *Server) currentGraph() *TypeGraph {
	project, built, _ := s.project.snapshot()
	if project == nil || built.graph == nil {
		return nil
	}

	s.mutex.Lock()
	open := make(map[string]*document, len(s.documents))
	for uri, doc := range s.documents {
		open[uri] = doc
	}
	s.mutex.Unlock()
	if len(open) == 0 {
		return built.graph
	}

	// The overlay is a copy rather than a mutation: the indexed graph outlives
	// the request, and a buffer's declarations must not leak into the next
	// answer about a file that was closed since.
	merged := newTypeGraph()
	for uri, file := range built.graph.byFile {
		if _, replaced := open[uri]; !replaced {
			merged.add(uri, file)
		}
	}
	for uri, doc := range open {
		path := filePathOf(doc.uri)
		if path == doc.uri {
			continue
		}
		s.mutex.Lock()
		file := symbolsOf(project, path, uri, doc.kind, doc.text, doc.found, doc.starts)
		s.mutex.Unlock()
		merged.add(uri, file)
	}
	return merged
}

func (s *Server) hoverRequest(message incoming) {
	var params textDocumentPositionParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		s.respondError(message.ID, codeInvalidParams, "unusable params: "+err.Error())
		return
	}
	symbol, _, at, resolved := s.resolveAt(params)
	if !resolved {
		// Null is the protocol's way of saying there is nothing here, and it
		// is the honest answer for a word that names no declaration.
		s.respond(message.ID, nil)
		return
	}
	marked := at
	s.respond(message.ID, Hover{
		Contents: MarkupContent{Kind: "markdown", Value: hoverFor(symbol)},
		Range:    &marked,
	})
}

func (s *Server) definitionRequest(message incoming) {
	var params textDocumentPositionParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		s.respondError(message.ID, codeInvalidParams, "unusable params: "+err.Error())
		return
	}
	symbol, _, _, resolved := s.resolveAt(params)
	if !resolved {
		s.respond(message.ID, []Location{})
		return
	}
	s.respond(message.ID, []Location{{URI: symbol.URI, Range: symbol.Range}})
}

func (s *Server) referencesRequest(message incoming) {
	var params referenceParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		s.respondError(message.ID, codeInvalidParams, "unusable params: "+err.Error())
		return
	}
	symbol, _, _, resolved := s.resolveAt(textDocumentPositionParams{
		TextDocument: params.TextDocument, Position: params.Position,
	})
	if !resolved {
		s.respond(message.ID, []Location{})
		return
	}
	project, _, _ := s.project.snapshot()
	s.respond(message.ID, referencesFor(referenceContext{
		project: project,
		graph:   s.currentGraph(),
		open:    s.openText(),
	}, symbol, params.Context.IncludeDeclaration))
}

// openText is the buffer of every open document, so a scan reads what the
// developer sees rather than what was last saved.
func (s *Server) openText() map[string]string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	text := make(map[string]string, len(s.documents))
	for uri, doc := range s.documents {
		text[uri] = doc.text
	}
	return text
}

func (s *Server) completionRequest(message incoming) {
	var params textDocumentPositionParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		s.respondError(message.ID, codeInvalidParams, "unusable params: "+err.Error())
		return
	}
	s.mutex.Lock()
	doc := s.documents[params.TextDocument.URI]
	s.mutex.Unlock()
	if doc == nil {
		s.respond(message.ID, []CompletionItem{})
		return
	}

	s.mutex.Lock()
	text, starts, kind, found := doc.text, doc.starts, doc.kind, doc.found
	s.mutex.Unlock()

	graph := s.currentGraph()
	s.respond(message.ID, completionsAt(text, starts, params.Position, completionContext{
		dialect: kind,
		uri:     params.TextDocument.URI,
		graph:   graph,
		scope:   scopeAt(found, graph, params.TextDocument.URI),
	}))
}

// scopeAt is every binding of every declaration in the document.
//
// It is the document's bindings rather than the enclosing declaration's,
// because deciding which declaration contains an offset needs an end position
// the parser does not record. An offer from a neighbouring declaration costs a
// wrong entry in a list; narrowing it wrongly would hide the right one.
func scopeAt(found analysis, graph *TypeGraph, uri string) []Binding {
	var bindings []Binding
	if found.module == nil {
		return bindings
	}
	for _, declaration := range found.module.Declarations {
		if template, ok := declaration.(*sqlbind.TemplateDecl); ok {
			bindings = append(bindings, bindingsIn(template, graph, uri)...)
		}
	}
	return bindings
}

func (s *Server) inlayHintRequest(message incoming) {
	var params inlayHintParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		s.respondError(message.ID, codeInvalidParams, "unusable params: "+err.Error())
		return
	}
	s.mutex.Lock()
	doc := s.documents[params.TextDocument.URI]
	s.mutex.Unlock()
	if doc == nil {
		s.respond(message.ID, []InlayHint{})
		return
	}
	s.mutex.Lock()
	text, starts, found := doc.text, doc.starts, doc.found
	s.mutex.Unlock()

	s.respond(message.ID, inlayHints(found, text, starts, params.Range,
		s.currentGraph(), params.TextDocument.URI, s.options.HintFamilies))
}

func (s *Server) generatedForRequest(message incoming) {
	var params textDocumentPositionParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		s.respondError(message.ID, codeInvalidParams, "unusable params: "+err.Error())
		return
	}
	symbol, word, _, resolved := s.resolveAt(params)
	if !resolved {
		s.respond(message.ID, GeneratedFragment{
			Status:      "missing",
			Declaration: word,
			Message:     "there is no declaration here to show the generated code of",
		})
		return
	}
	project, _, _ := s.project.snapshot()
	s.respond(message.ID, generatedFor(project, symbol))
}

func (s *Server) routesRequest(message incoming) {
	project, _, _ := s.project.snapshot()
	s.respond(message.ID, routesOf(project))
}

func (s *Server) storyForRequest(message incoming) {
	var params textDocumentPositionParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		s.respondError(message.ID, codeInvalidParams, "unusable params: "+err.Error())
		return
	}
	symbol, word, _, resolved := s.resolveAt(params)
	if !resolved {
		symbol.Name = word
	}
	project, _, _ := s.project.snapshot()
	s.respond(message.ID, storyFor(project, symbol, resolved))
}
