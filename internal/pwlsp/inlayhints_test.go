package pwlsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hintProject writes a project whose bindings a hint can actually resolve: a
// statement with a declared output, and a component binding a call to it.
func hintProject(t *testing.T) (string, *Project, string, string) {
	t.Helper()
	root := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		return path
	}

	write("templates/data.pw.html", strings.Join([]string{
		"package widgets",
		"",
		"type Room {",
		"  id: int",
		"}",
		"",
		"external Rooms(): Room[]",
		"",
		"external Lookup(id: int): Room",
		"",
	}, "\n"))

	page := write("templates/page.pw.html", strings.Join([]string{
		"package widgets",
		"",
		"export component Page(q: string): html {",
		"  {val room = Lookup(1)}",
		"  <ul>",
		"    {for item in Rooms()}",
		"      <li>{item.id}</li>",
		"    {/for}",
		"  </ul>",
		"}",
		"",
	}, "\n"))

	project := &Project{Root: root, Sources: []SourceDir{
		{Purpose: "generate.templates", Dir: filepath.Join(root, "templates"), Kinds: DialectsFor("generate.templates")},
	}}
	source, err := os.ReadFile(page)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return root, project, uriOf(page), string(source)
}

func hintsOf(t *testing.T, families map[BindingKind]bool) []InlayHint {
	t.Helper()
	_, project, uri, text := hintProject(t)
	graph := buildIndex(project).graph
	starts := newLineStarts(text)
	found := analyze("page.pw.html", dialectHTML, text, starts)
	whole := Range{End: Position{Line: len(starts)}}
	return inlayHints(found, text, starts, whole, graph, uri, families)
}

func labelled(hints []InlayHint, label string) bool {
	for _, hint := range hints {
		if strings.Contains(hint.Label, label) {
			return true
		}
	}
	return false
}

func TestAValBindingIsAnnotatedWithWhatItHolds(t *testing.T) {
	// The source never writes this type, which is the whole reason the hint
	// exists: the alternative is reading the generated Go.
	hints := hintsOf(t, defaultHintFamilies())

	if !labelled(hints, "Room") {
		t.Fatalf("hints = %+v, want the val binding's type", hints)
	}
}

func TestALoopVariableIsAnnotatedWithTheElementType(t *testing.T) {
	// A loop binds an element, so the array suffix comes off.
	hints := hintsOf(t, defaultHintFamilies())

	for _, hint := range hints {
		if strings.HasSuffix(hint.Label, "[]") {
			t.Fatalf("hint %q annotates the collection, not the element", hint.Label)
		}
	}
	if len(hints) < 2 {
		t.Fatalf("hints = %+v, want the binding and the loop variable", hints)
	}
}

func TestAParameterIsNotAnnotated(t *testing.T) {
	// It writes its own type. A hint there would repeat the source back.
	families := defaultHintFamilies()
	families[bindingParameter] = true

	for _, hint := range hintsOf(t, families) {
		if strings.Contains(hint.Tooltip, "a parameter of") {
			t.Fatalf("a parameter was annotated: %+v", hint)
		}
	}
}

func TestAFamilySwitchedOffProducesNoHint(t *testing.T) {
	hints := hintsOf(t, map[BindingKind]bool{bindingLoop: true})

	for _, hint := range hints {
		if strings.Contains(hint.Tooltip, "val binding") {
			t.Fatalf("a switched-off family produced %+v", hint)
		}
	}
}

func TestAnUnresolvedBindingIsNotGuessedAt(t *testing.T) {
	// A binding of an expression this server does not evaluate has no type to
	// state, and inventing one would be worse than saying nothing.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	text := "package p\n\nexport component C(): html {\n  {val n = 1 + 2}\n  <p>{n}</p>\n}\n"
	path := filepath.Join(root, "templates", "c.pw.html")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	project := &Project{Root: root, Sources: []SourceDir{
		{Purpose: "generate.templates", Dir: filepath.Join(root, "templates"), Kinds: DialectsFor("generate.templates")},
	}}
	starts := newLineStarts(text)
	found := analyze("c.pw.html", dialectHTML, text, starts)

	hints := inlayHints(found, text, starts, Range{End: Position{Line: 99}}, buildIndex(project).graph, uriOf(path), defaultHintFamilies())

	if len(hints) != 0 {
		t.Fatalf("hints = %+v, want none for an expression with no resolved type", hints)
	}
}

func TestOnlyTheRequestedRangeIsAnswered(t *testing.T) {
	// A client asks about the lines it is showing, and answering the whole
	// document would be work nobody sees.
	_, project, uri, text := hintProject(t)
	starts := newLineStarts(text)
	found := analyze("page.pw.html", dialectHTML, text, starts)

	hints := inlayHints(found, text, starts, Range{Start: Position{Line: 0}, End: Position{Line: 1}},
		buildIndex(project).graph, uri, defaultHintFamilies())

	if len(hints) != 0 {
		t.Fatalf("hints = %+v, want none in the header lines", hints)
	}
}

func TestALongTypeIsShortenedRatherThanReflowingTheLine(t *testing.T) {
	long := strings.Repeat("Name", 20)

	if shortened := shorten(long); len([]rune(shortened)) > maxHintLabel {
		t.Fatalf("shortened = %q, still longer than the cap", shortened)
	}
	if shorten("Room") != "Room" {
		t.Fatal("a short label was changed")
	}
}
