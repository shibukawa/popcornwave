package pwlsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const configFixture = `[project]
name = "fixture"

# Each purpose reads only the directories it lists.
[generate]
handlers = ["handlers"]
templates = ["templates"]
queries = ["queries"]
config = []
`

// strayProject writes a project with a template no purpose compiles, which is
// the one finding this server produces that has a mechanical repair.
func strayProject(t *testing.T) (string, *Project, *document) {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{"handlers", "templates", "queries", "scratch"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "popcornweb.toml"), []byte(configFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(root, "scratch", "draft.pw.html")
	text := "export component Draft(): html {\n  <p>x</p>\n}\n"
	if err := os.WriteFile(stray, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	project := &Project{
		Root:       root,
		ConfigPath: filepath.Join(root, "popcornweb.toml"),
		Sources: []SourceDir{
			{Purpose: "generate.templates", Dir: filepath.Join(root, "templates"), Kinds: DialectsFor("generate.templates")},
			{Purpose: "generate.queries", Dir: filepath.Join(root, "queries"), Kinds: DialectsFor("generate.queries")},
		},
	}
	starts := newLineStarts(text)
	doc := &document{
		uri:    uriOf(stray),
		kind:   dialectHTML,
		text:   text,
		starts: starts,
		found:  analyze("draft.pw.html", dialectHTML, text, starts),
	}
	return root, project, doc
}

func strayFinding(project *Project, doc *document) Diagnostic {
	message, _ := project.strayMessage(filePathOf(doc.uri))
	return Diagnostic{
		Range:    doc.starts.rangeAt(doc.text, 0),
		Severity: severityWarning,
		Source:   "pw",
		Message:  message,
	}
}

func TestTheFixListsTheDirectoryTheDeveloperAlreadyChose(t *testing.T) {
	// The requirement names two repairs for this finding. Moving the file is
	// a judgement about the layout, and an action that picked a destination
	// would be choosing for the developer; listing the directory they already
	// put it in is not a choice.
	root, project, doc := strayProject(t)

	actions := codeActionsFor(project, doc, Range{End: Position{Line: 99}}, []Diagnostic{strayFinding(project, doc)})

	if len(actions) != 1 {
		t.Fatalf("actions = %+v, want the one repair", actions)
	}
	if actions[0].Kind != kindQuickFix {
		t.Fatalf("kind = %q, want a quickfix beside the finding", actions[0].Kind)
	}
	if !strings.Contains(actions[0].Title, "scratch") || !strings.Contains(actions[0].Title, "templates") {
		t.Fatalf("title = %q, want the directory and the key", actions[0].Title)
	}

	edits := actions[0].Edit.Changes[uriOf(filepath.Join(root, "popcornweb.toml"))]
	if len(edits) != 1 {
		t.Fatalf("edits = %+v, want one", edits)
	}
	if edits[0].NewText != ` ["templates", "scratch"]` {
		t.Fatalf("new text = %q", edits[0].NewText)
	}
}

func TestTheFixEditsOneLineRatherThanTheDocument(t *testing.T) {
	// The file is the developer's, full of the comments the scaffold wrote. A
	// re-serialized TOML would hand it back reformatted.
	_, project, doc := strayProject(t)

	edits := codeActionsFor(project, doc, Range{End: Position{Line: 99}}, []Diagnostic{strayFinding(project, doc)})[0].Edit
	for _, change := range edits.Changes {
		if change[0].Range.Start.Line != change[0].Range.End.Line {
			t.Fatalf("the edit spans %+v, want one line", change[0].Range)
		}
	}
}

func TestTheDialectDecidesWhichPurposeIsOffered(t *testing.T) {
	root, project, _ := strayProject(t)
	stray := filepath.Join(root, "scratch", "rooms.pw.sql")
	text := "package q\nexport statement F(): sql.exec {\n  DELETE FROM t\n}\n"
	if err := os.WriteFile(stray, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	starts := newLineStarts(text)
	doc := &document{uri: uriOf(stray), kind: dialectSQL, text: text, starts: starts,
		found: analyze("rooms.pw.sql", dialectSQL, text, starts)}

	actions := codeActionsFor(project, doc, Range{End: Position{Line: 99}}, []Diagnostic{strayFinding(project, doc)})

	if len(actions) != 1 || !strings.Contains(actions[0].Title, "queries") {
		t.Fatalf("actions = %+v, want the queries purpose", actions)
	}
}

func TestNoActionIsOfferedWithoutAFinding(t *testing.T) {
	// The gate requirement:editor-code-actions sets: the editor offers nothing
	// api:cli-generate would not have complained about first.
	_, project, doc := strayProject(t)

	if actions := codeActionsFor(project, doc, Range{End: Position{Line: 99}}, nil); len(actions) != 0 {
		t.Fatalf("actions = %+v, want none", actions)
	}
}

func TestASyntaxErrorHasNoAction(t *testing.T) {
	// What to write is the developer's answer, and an action that guessed
	// would be the invented code the requirement excludes.
	_, project, doc := strayProject(t)
	syntax := Diagnostic{Severity: severityError, Source: "pw", Message: "missing closing tag </p>"}

	if actions := codeActionsFor(project, doc, Range{End: Position{Line: 99}}, []Diagnostic{syntax}); len(actions) != 0 {
		t.Fatalf("actions = %+v, want none for a syntax error", actions)
	}
}

func TestAPageTreeFindingHasNoDirectoryToList(t *testing.T) {
	// That finding is about the file's name rather than its directory, so
	// listing the directory would repair nothing.
	_, project, doc := strayProject(t)
	finding := Diagnostic{
		Severity: severityWarning,
		Source:   "pw",
		Message:  "pages/rooms/card.pw.html is inside a page tree but is not page.pw.html, layout.pw.html, or document.pw.html, so nothing compiles it",
	}

	if actions := codeActionsFor(project, doc, Range{End: Position{Line: 99}}, []Diagnostic{finding}); len(actions) != 0 {
		t.Fatalf("actions = %+v, want none", actions)
	}
}

func TestADirectoryAlreadyListedIsNotOfferedAgain(t *testing.T) {
	entry, added := withDirectory(`["templates", "scratch"]`, "scratch")

	if added {
		t.Fatalf("offered to add a directory already there: %q", entry)
	}
}

func TestAValueSpreadOverSeveralLinesIsLeftAlone(t *testing.T) {
	// Rewriting it would reformat what the developer wrote, which is worse
	// than not offering the fix.
	config := "[generate]\ntemplates = [\n  \"templates\",\n]\n"

	if _, _, ok := purposeEntry(config, "templates"); ok {
		t.Fatal("a multi-line array was taken as editable")
	}
}

func TestAnAbsentPurposeKeyIsNotInvented(t *testing.T) {
	config := "[generate]\nhandlers = [\"handlers\"]\n"

	if _, _, ok := purposeEntry(config, "templates"); ok {
		t.Fatal("a key the file does not have was offered an edit")
	}
}

func TestWithNoProjectThereIsNothingToRepair(t *testing.T) {
	_, _, doc := strayProject(t)

	if actions := codeActionsFor(nil, doc, Range{End: Position{Line: 99}}, []Diagnostic{{Severity: severityWarning}}); len(actions) != 0 {
		t.Fatalf("actions = %+v, want none", actions)
	}
}
