package pwlsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/internal/pwgen"
)

// projectSession drives a connection with a loader attached, and reports both
// the messages and how many times the loader ran.
func projectSession(t *testing.T, load Loader, messages ...any) ([]map[string]any, *int) {
	t.Helper()
	var input bytes.Buffer
	for _, message := range messages {
		if err := writeMessage(&input, message); err != nil {
			t.Fatalf("framing the input: %v", err)
		}
	}
	loads := 0
	counted := func(start string) (*Project, error) {
		loads++
		return load(start)
	}

	var output bytes.Buffer
	NewServer(&output, Options{Load: counted}).Serve(&input)

	var decoded []map[string]any
	reader := bufio.NewReader(&output)
	for {
		body, err := readMessage(reader)
		if err == io.EOF {
			return decoded, &loads
		}
		if err != nil {
			t.Fatalf("reading the output: %v", err)
		}
		var message map[string]any
		if err := json.Unmarshal(body, &message); err != nil {
			t.Fatalf("decoding the output: %v", err)
		}
		decoded = append(decoded, message)
	}
}

func initializeAt(root string) any {
	return request(1, "initialize", map[string]any{"rootUri": uriOf(root)})
}

func symbolNames(t *testing.T, messages []map[string]any, id float64) []string {
	t.Helper()
	reply := resultOf(t, messages, id)
	if reply["error"] != nil {
		t.Fatalf("reply = %+v, want a result", reply)
	}
	var found []string
	for _, entry := range reply["result"].([]any) {
		found = append(found, entry.(map[string]any)["name"].(string))
	}
	return found
}

func TestWorkspaceSymbolsAnswerFromTheProjectIndex(t *testing.T) {
	root, project := writeProject(t)
	load := func(string) (*Project, error) { return project, nil }

	messages, loads := projectSession(t, load,
		initializeAt(root),
		notify("initialized", map[string]any{}),
		request(2, "workspace/symbol", map[string]any{"query": "room"}),
		notify("exit", nil),
	)

	if *loads != 1 {
		t.Fatalf("the project was loaded %d times, want once", *loads)
	}
	found := symbolNames(t, messages, 2)
	if len(found) != 2 || found[0] != "Room" || found[1] != "RoomByID" {
		t.Fatalf("symbols = %v, want the two room declarations", found)
	}
}

func TestAnUnsavedDeclarationIsFoundBeforeTheSave(t *testing.T) {
	// The acceptance criterion of requirement:editor-workspace-symbols: an
	// index of files on disk would answer about the file, not the buffer.
	root, project := writeProject(t)
	uri := uriOf(filepath.Join(root, "templates", "card.pw.html"))

	messages, _ := projectSession(t, func(string) (*Project, error) { return project, nil },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
			"uri": uri, "languageId": "pw-html", "version": 1,
			"text": "export component Card(label: string): html {\n  <p>{label}</p>\n}\n\nexport component Badge(): html {\n  <b>new</b>\n}\n",
		}}),
		request(2, "workspace/symbol", map[string]any{"query": "badge"}),
		notify("exit", nil),
	)

	if found := symbolNames(t, messages, 2); len(found) != 1 || found[0] != "Badge" {
		t.Fatalf("symbols = %v, want the unsaved declaration", found)
	}
}

func TestAnOpenDocumentOutsideEveryPurposeIsNotAWorkspaceSymbol(t *testing.T) {
	// The search covers the scope api:cli-generate reads. Having a file open
	// does not put it in that scope.
	root, project := writeProject(t)

	messages, _ := projectSession(t, func(string) (*Project, error) { return project, nil },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
			"uri": uriOf(filepath.Join(root, "scratch", "draft.pw.html")), "languageId": "pw-html", "version": 1,
			"text": "export component Draft(): html {\n  <p>x</p>\n}\n",
		}}),
		request(2, "workspace/symbol", map[string]any{"query": "draft"}),
		notify("exit", nil),
	)

	if found := symbolNames(t, messages, 2); len(found) != 0 {
		t.Fatalf("symbols = %v, want none", found)
	}
}

func TestADocumentOutsideTheProjectStillGetsSyntaxDiagnostics(t *testing.T) {
	// Parse-only is the floor, not a mode: a file the project does not own is
	// still a file the developer is editing.
	root, project := writeProject(t)
	uri := uriOf(filepath.Join(root, "scratch", "draft.pw.html"))

	messages, _ := projectSession(t, func(string) (*Project, error) { return project, nil },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
			"uri": uri, "languageId": "pw-html", "version": 1,
			"text": "export component Draft(): html {\n  <p>unclosed\n}\n",
		}}),
		notify("exit", nil),
	)

	diagnostics, published := diagnosticsFor(messages, uri)
	if !published {
		t.Fatal("nothing was published for a document outside the project")
	}
	syntax := 0
	for _, entry := range diagnostics {
		if entry.(map[string]any)["severity"].(float64) == severityError {
			syntax++
		}
	}
	if syntax != 1 {
		t.Fatalf("diagnostics = %+v, want the syntax error among them", diagnostics)
	}
}

func TestASourceNoPurposeCompilesIsReportedInGenerationsOwnWords(t *testing.T) {
	// decision:shared-check-catalog: the editor reports the condition
	// api:cli-generate reports, in the words api:cli-generate uses. The text
	// is pwgen's, so a change there reaches both at once.
	root, project := writeProject(t)
	uri := uriOf(filepath.Join(root, "scratch", "draft.pw.html"))

	messages, _ := projectSession(t, func(string) (*Project, error) { return project, nil },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
			"uri": uri, "languageId": "pw-html", "version": 1,
			"text": "export component Draft(): html {\n  <p>x</p>\n}\n",
		}}),
		notify("exit", nil),
	)

	diagnostics, _ := diagnosticsFor(messages, uri)
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want the one project finding", diagnostics)
	}
	finding := diagnostics[0].(map[string]any)
	if finding["severity"].(float64) != severityWarning {
		t.Fatalf("severity = %v, want a warning: the file is valid and nothing reads it", finding["severity"])
	}
	wanted, _ := pwgen.StrayTemplateMessage("scratch/draft.pw.html", "draft.pw.html", pwgen.SourcePurposes{})
	if finding["message"] != wanted {
		t.Fatalf("message = %q, want generation's own %q", finding["message"], wanted)
	}
}

func TestATemplateInsideAPurposeIsNotReported(t *testing.T) {
	root, project := writeProject(t)
	uri := uriOf(filepath.Join(root, "templates", "card.pw.html"))

	messages, _ := projectSession(t, func(string) (*Project, error) { return project, nil },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
			"uri": uri, "languageId": "pw-html", "version": 1,
			"text": "export component Card(label: string): html {\n  <p>{label}</p>\n}\n",
		}}),
		notify("exit", nil),
	)

	if diagnostics, _ := diagnosticsFor(messages, uri); len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none", diagnostics)
	}
}

func TestATemplateAPageTreeDoesNotCompileIsReported(t *testing.T) {
	// A page tree compiles only the names it reserves, so a card.pw.html
	// inside one is stray although its directory serves a purpose.
	root, project := writeProject(t)
	stray := filepath.Join(root, "pages", "rooms", "card.pw.html")
	if err := os.WriteFile(stray, []byte("export component Card(): html {\n  <p>x</p>\n}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	uri := uriOf(stray)

	messages, _ := projectSession(t, func(string) (*Project, error) { return project, nil },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
			"uri": uri, "languageId": "pw-html", "version": 1,
			"text": "export component Card(): html {\n  <p>x</p>\n}\n",
		}}),
		notify("exit", nil),
	)

	diagnostics, _ := diagnosticsFor(messages, uri)
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want the page tree finding", diagnostics)
	}
	if message := diagnostics[0].(map[string]any)["message"].(string); !strings.Contains(message, "page tree") {
		t.Fatalf("message = %q, want the page tree wording", message)
	}
}

func TestNoProjectMeansNoProjectFinding(t *testing.T) {
	// Parse-only reports what it can prove. A file with no project above it is
	// not misplaced; there is nothing for it to be misplaced from.
	root := t.TempDir()
	uri := uriOf(filepath.Join(root, "draft.pw.html"))

	messages, _ := projectSession(t, func(string) (*Project, error) { return nil, ErrNoProject },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
			"uri": uri, "languageId": "pw-html", "version": 1,
			"text": "export component Draft(): html {\n  <p>x</p>\n}\n",
		}}),
		notify("exit", nil),
	)

	if diagnostics, _ := diagnosticsFor(messages, uri); len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none", diagnostics)
	}
}

func TestNoProjectIsAnnouncedOnceAndAnswersNoSymbols(t *testing.T) {
	messages, _ := projectSession(t, func(string) (*Project, error) { return nil, ErrNoProject },
		initializeAt(t.TempDir()),
		notify("initialized", map[string]any{}),
		request(2, "workspace/symbol", map[string]any{"query": "anything"}),
		notify("exit", nil),
	)

	announcements := 0
	for _, message := range messages {
		if message["method"] == "window/logMessage" {
			announcements++
		}
	}
	if announcements != 1 {
		t.Fatalf("announcements = %d, want exactly one", announcements)
	}
	if found := symbolNames(t, messages, 2); len(found) != 0 {
		t.Fatalf("symbols = %v, want none rather than a guess", found)
	}
}

func TestAProjectThatWillNotLoadIsADiagnosticOnItsConfiguration(t *testing.T) {
	// api:cli-lsp: report the load error as a workspace diagnostic and keep
	// serving syntax analysis. The developer's next move is to open that file.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "popcornweb.toml"), []byte("broken"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	messages, _ := projectSession(t, func(string) (*Project, error) { return nil, errors.New("unknown key wombat") },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		notify("exit", nil),
	)

	diagnostics, published := diagnosticsFor(messages, uriOf(filepath.Join(root, "popcornweb.toml")))
	if !published || len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want the load error", diagnostics)
	}
	if message := diagnostics[0].(map[string]any)["message"].(string); message == "" {
		t.Fatal("the diagnostic carries no message")
	}
}

func TestAConfigurationChangeReloadsWithoutARestart(t *testing.T) {
	root, project := writeProject(t)
	// The second load sees a project that lists only the queries purpose,
	// which is what an edit removing a purpose would produce.
	loads := 0
	load := func(string) (*Project, error) {
		loads++
		if loads == 1 {
			return project, nil
		}
		trimmed := *project
		trimmed.Sources = project.Sources[2:3]
		return &trimmed, nil
	}

	messages, _ := projectSession(t, load,
		initializeAt(root),
		notify("initialized", map[string]any{}),
		request(2, "workspace/symbol", map[string]any{"query": "card"}),
		notify("workspace/didChangeWatchedFiles", map[string]any{"changes": []any{
			map[string]any{"uri": uriOf(filepath.Join(root, "popcornweb.toml")), "type": 2},
		}}),
		request(3, "workspace/symbol", map[string]any{"query": "card"}),
		notify("exit", nil),
	)

	if found := symbolNames(t, messages, 2); len(found) != 1 {
		t.Fatalf("before the reload = %v, want the template", found)
	}
	if found := symbolNames(t, messages, 3); len(found) != 0 {
		t.Fatalf("after the reload = %v, want none: the purpose is gone", found)
	}
}

func TestAChangeToAnotherWatchedFileDoesNotReload(t *testing.T) {
	// The client may watch more than this server cares about, and a reload
	// walks the tree.
	root, project := writeProject(t)

	_, loads := projectSession(t, func(string) (*Project, error) { return project, nil },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		notify("workspace/didChangeWatchedFiles", map[string]any{"changes": []any{
			map[string]any{"uri": uriOf(filepath.Join(root, "go.mod")), "type": 2},
		}}),
		notify("exit", nil),
	)

	if *loads != 1 {
		t.Fatalf("the project was loaded %d times, want once", *loads)
	}
}

func TestAServerWithNoLoaderStaysInParseOnlyMode(t *testing.T) {
	// The zero configuration is the safe one: no loader means no resolved
	// answers rather than a walk of whatever directory it was started in.
	var output bytes.Buffer
	var input bytes.Buffer
	_ = writeMessage(&input, request(1, "initialize", map[string]any{}))
	_ = writeMessage(&input, notify("initialized", map[string]any{}))
	_ = writeMessage(&input, request(2, "workspace/symbol", map[string]any{"query": ""}))
	_ = writeMessage(&input, notify("exit", nil))
	NewServer(&output, Options{}).Serve(&input)

	reader := bufio.NewReader(&output)
	var messages []map[string]any
	for {
		body, err := readMessage(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading the output: %v", err)
		}
		var message map[string]any
		_ = json.Unmarshal(body, &message)
		messages = append(messages, message)
	}
	if found := symbolNames(t, messages, 2); len(found) != 0 {
		t.Fatalf("symbols = %v, want none", found)
	}
}
