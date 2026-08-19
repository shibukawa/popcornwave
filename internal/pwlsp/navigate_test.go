package pwlsp

import (
	"path/filepath"
	"strings"
	"testing"
)

func openDocument(uri, language, text string) any {
	return notify("textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": uri, "languageId": language, "version": 1, "text": text,
	}})
}

func at(uri string, line, character int) map[string]any {
	return map[string]any{
		"textDocument": map[string]any{"uri": uri},
		"position":     map[string]any{"line": line, "character": character},
	}
}

const cardSource = "package widgets\n\ntype Room {\n  id: int\n}\n\nexport component Card(room: Room): html {\n  <p>{room.id}</p>\n}\n"

func TestHoverAnswersAboutTheDeclarationUnderTheCaret(t *testing.T) {
	root, project := graphProject(t)
	uri := uriOf(filepath.Join(root, "templates", "card.pw.html"))

	messages, _ := projectSession(t, func(string) (*Project, error) { return project, nil },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		openDocument(uri, "pw-html", cardSource),
		// The Room in the Card parameter list, on the line declaring Card.
		request(2, "textDocument/hover", at(uri, 6, 28)),
		notify("exit", nil),
	)

	rendered := resultOf(t, messages, 2)["result"].(map[string]any)
	value := rendered["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(value, "type Room") {
		t.Fatalf("hover = %q, want the record it names", value)
	}
	if !strings.Contains(value, "`id`") {
		t.Fatalf("hover = %q, want the record's fields", value)
	}
}

func TestHoverOnNothingIsNullRatherThanAnEmptyBox(t *testing.T) {
	root, project := graphProject(t)
	uri := uriOf(filepath.Join(root, "templates", "card.pw.html"))

	messages, _ := projectSession(t, func(string) (*Project, error) { return project, nil },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		openDocument(uri, "pw-html", cardSource),
		request(2, "textDocument/hover", at(uri, 1, 0)),
		notify("exit", nil),
	)

	if reply := resultOf(t, messages, 2); reply["result"] != nil {
		t.Fatalf("result = %+v, want null", reply["result"])
	}
}

func TestDefinitionJumpsToTheDeclaringFile(t *testing.T) {
	// The acceptance requirement:editor-navigation states for a project with
	// no generated output: template to template still navigates.
	root, project := graphProject(t)
	page := uriOf(filepath.Join(root, "templates", "page.pw.html"))
	card := uriOf(filepath.Join(root, "templates", "card.pw.html"))

	messages, _ := projectSession(t, func(string) (*Project, error) { return project, nil },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		openDocument(page, "pw-html", "package pages\nimport \"app/widgets\"\n\nexport component Page(): html {\n  <Card />\n}\n"),
		request(2, "textDocument/definition", at(page, 4, 4)),
		notify("exit", nil),
	)

	locations := resultOf(t, messages, 2)["result"].([]any)
	if len(locations) != 1 {
		t.Fatalf("locations = %+v, want the one declaration", locations)
	}
	if locations[0].(map[string]any)["uri"] != card {
		t.Fatalf("uri = %v, want %s", locations[0].(map[string]any)["uri"], card)
	}
}

func TestDefinitionOnAnUnresolvedNameIsEmptyRatherThanWrong(t *testing.T) {
	root, project := graphProject(t)
	uri := uriOf(filepath.Join(root, "templates", "card.pw.html"))

	messages, _ := projectSession(t, func(string) (*Project, error) { return project, nil },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		openDocument(uri, "pw-html", "package widgets\n\nexport component Card(): html {\n  <Nowhere />\n}\n"),
		request(2, "textDocument/definition", at(uri, 3, 4)),
		notify("exit", nil),
	)

	if locations := resultOf(t, messages, 2)["result"].([]any); len(locations) != 0 {
		t.Fatalf("locations = %+v, want none", locations)
	}
}

func TestAnUnsavedDeclarationIsResolvedFromTheBuffer(t *testing.T) {
	// The index holds the file; the buffer is what the developer is looking
	// at. A name added and not saved has to resolve, and to its new position.
	root, project := graphProject(t)
	uri := uriOf(filepath.Join(root, "templates", "card.pw.html"))

	messages, _ := projectSession(t, func(string) (*Project, error) { return project, nil },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		openDocument(uri, "pw-html", cardSource+"\nexport component Badge(): html {\n  <b>new</b>\n}\n"),
		request(2, "textDocument/hover", at(uri, 10, 18)),
		notify("exit", nil),
	)

	rendered := resultOf(t, messages, 2)["result"]
	if rendered == nil {
		t.Fatal("a declaration added in the buffer did not resolve")
	}
	value := rendered.(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(value, "component Badge()") {
		t.Fatalf("hover = %q, want the unsaved declaration", value)
	}
}

func TestClosingADocumentRestoresTheIndexedAnswer(t *testing.T) {
	// The overlay is per-request rather than a mutation of the index, so a
	// buffer's declarations must not outlive the buffer.
	root, project := graphProject(t)
	uri := uriOf(filepath.Join(root, "templates", "card.pw.html"))

	messages, _ := projectSession(t, func(string) (*Project, error) { return project, nil },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		openDocument(uri, "pw-html", cardSource+"\nexport component Badge(): html {\n  <b>new</b>\n}\n"),
		notify("textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": uri}}),
		openDocument(uri, "pw-html", cardSource),
		request(2, "textDocument/definition", at(uri, 6, 18)),
		notify("exit", nil),
	)

	if locations := resultOf(t, messages, 2)["result"].([]any); len(locations) != 1 {
		t.Fatalf("locations = %+v, want the indexed declaration", locations)
	}
}

func TestWithNoProjectNothingResolves(t *testing.T) {
	// Parse-only reports what it can prove. Resolving a name needs to know
	// which files are in scope, and with no project nothing is.
	root := t.TempDir()
	uri := uriOf(filepath.Join(root, "card.pw.html"))

	messages, _ := projectSession(t, func(string) (*Project, error) { return nil, ErrNoProject },
		initializeAt(root),
		notify("initialized", map[string]any{}),
		openDocument(uri, "pw-html", cardSource),
		request(2, "textDocument/hover", at(uri, 6, 28)),
		notify("exit", nil),
	)

	if reply := resultOf(t, messages, 2); reply["result"] != nil {
		t.Fatalf("result = %+v, want null rather than a guess", reply["result"])
	}
}

func TestTheCapabilitiesNameWhatIsServed(t *testing.T) {
	messages, _ := session(t,
		request(1, "initialize", map[string]any{}),
		request(2, "shutdown", nil),
		notify("exit", nil),
	)

	capabilities := resultOf(t, messages, 1)["result"].(map[string]any)["capabilities"].(map[string]any)
	for _, capability := range []string{
		"hoverProvider", "definitionProvider", "referencesProvider", "inlayHintProvider",
	} {
		if capabilities[capability] != true {
			t.Errorf("%s is not declared", capability)
		}
	}
	if _, declared := capabilities["completionProvider"]; !declared {
		t.Error("completionProvider is not declared")
	}
	// Still not served, and still not declared: a client that believes it will
	// wait for an answer that never comes. Rename is deferred to
	// requirement:declaration-rename rather than implemented here.
	for _, capability := range []string{"renameProvider", "codeActionProvider"} {
		if _, declared := capabilities[capability]; declared {
			t.Errorf("%s is declared and not served", capability)
		}
	}
}
