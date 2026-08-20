package pwlsp

// The subset of the LSP type set this server exchanges.
//
// Only the fields the server reads or sends are declared. A field absent here
// is one requirement:pw-language-server does not answer yet, and adding it is
// how a new capability arrives, rather than by widening a generated type set
// nothing reconciles against what the server does.

import (
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// syncFull is the TextDocumentSyncKind that sends a whole document on every
// change.
const syncFull = 1

type initializeParams struct {
	RootURI          string            `json:"rootUri"`
	RootPath         string            `json:"rootPath"`
	WorkspaceFolders []workspaceFolder `json:"workspaceFolders"`
}

type workspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// workspaceRoot is the root the client named, preferring the folder list a
// modern client sends over the fields kept for older ones.
func (p initializeParams) workspaceRoot() string {
	if len(p.WorkspaceFolders) > 0 {
		return filePathOf(p.WorkspaceFolders[0].URI)
	}
	if p.RootURI != "" {
		return filePathOf(p.RootURI)
	}
	return p.RootPath
}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
	ServerInfo   serverInfo         `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type serverCapabilities struct {
	TextDocumentSync        textDocumentSyncOptions `json:"textDocumentSync"`
	DocumentSymbolProvider  bool                    `json:"documentSymbolProvider"`
	WorkspaceSymbolProvider bool                    `json:"workspaceSymbolProvider"`
	HoverProvider           bool                    `json:"hoverProvider"`
	DefinitionProvider      bool                    `json:"definitionProvider"`
	ReferencesProvider      bool                    `json:"referencesProvider"`
	InlayHintProvider       bool                    `json:"inlayHintProvider"`
	CompletionProvider      *completionOptions      `json:"completionProvider,omitempty"`
	CodeActionProvider      bool                    `json:"codeActionProvider"`
	RenameProvider          bool                    `json:"renameProvider"`
}

// completionOptions declares what a client should re-ask on. The characters
// are the two that open a position with a closed set of answers: a tag and an
// expression.
type completionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters"`
}

type textDocumentSyncOptions struct {
	OpenClose bool         `json:"openClose"`
	Change    int          `json:"change"`
	Save      *saveOptions `json:"save,omitempty"`
}

type saveOptions struct {
	IncludeText bool `json:"includeText"`
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type didChangeParams struct {
	TextDocument   versionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []contentChange                 `json:"contentChanges"`
}

// contentChange carries only Text, because the server declares full sync and
// therefore never receives a ranged change.
type contentChange struct {
	Text string `json:"text"`
}

type didCloseParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type documentSymbolParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

// textDocumentPositionParams is the shape every request about a caret takes.
type textDocumentPositionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type referenceParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      referenceOptions       `json:"context"`
}

type referenceOptions struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

type renameParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	NewName      string                 `json:"newName"`
}

type codeActionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      codeActionContext      `json:"context"`
}

// codeActionContext carries the findings the client is showing at the range.
// They are taken from the request rather than recomputed, so an action is
// always attached to a finding the developer can see.
type codeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type inlayHintParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
}

type workspaceSymbolParams struct {
	Query string `json:"query"`
}

// didChangeWatchedFilesParams carries the files the client watches on the
// server's behalf. The client registers the watch, so this server needs no
// dynamic registration and no file watcher of its own.
type didChangeWatchedFilesParams struct {
	Changes []fileEvent `json:"changes"`
}

type fileEvent struct {
	URI string `json:"uri"`
	// Type is created, changed, or deleted. All three reload: a deleted
	// popcornweb.toml is a project that stopped existing, which is as much a
	// change to the model as an edited one.
	Type int `json:"type"`
}

// namesProjectConfig reports whether any change is to a project configuration.
func (p didChangeWatchedFilesParams) namesProjectConfig() bool {
	for _, change := range p.Changes {
		if strings.HasSuffix(filePathOf(change.URI), "popcornweb.toml") {
			return true
		}
	}
	return false
}

// window/logMessage severities. Only the informational one is used: everything
// this server has to say about its own state is information, and a warning
// would suggest the developer did something wrong by opening a file.
const logInfo = 3

type logMessageParams struct {
	Type    int    `json:"type"`
	Message string `json:"message"`
}

type publishDiagnosticsParams struct {
	URI     string `json:"uri"`
	Version *int   `json:"version,omitempty"`
	// Diagnostics is never omitted: an empty list is how a client is told the
	// findings it holds are gone, and omitting it would leave them on screen.
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// filePathOf turns a file URI into a path. A URI this server cannot decode is
// returned unchanged, because it is only ever used for a name in a message.
func filePathOf(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" {
		return uri
	}
	name := parsed.Path
	// A Windows URI carries the drive letter as the first path segment.
	if len(name) > 2 && name[0] == '/' && name[2] == ':' {
		name = name[1:]
	}
	return name
}

// fileNameOf is the name a diagnostic is reported against. The parsers put it
// in their message, so it is the file's own name rather than the whole URI,
// which would make a message unreadable in a client that shows it verbatim.
func fileNameOf(uri string) string {
	name := filePathOf(uri)
	if index := strings.LastIndexAny(name, `/\`); index >= 0 {
		return name[index+1:]
	}
	return path.Base(name)
}

// uriOf turns a path into a file URI. Only the characters a path can hold and
// a URI cannot are escaped; a component is escaped as a path segment, so the
// separators survive as separators.
func uriOf(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	slashed := filepath.ToSlash(absolute)
	// A Windows path starts at a drive letter rather than at a separator, and a
	// file URI needs the leading slash either way.
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	segments := strings.Split(slashed, "/")
	for index, segment := range segments {
		segments[index] = url.PathEscape(segment)
	}
	return "file://" + strings.Join(segments, "/")
}
