package pwlsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// graphProject writes a project whose declarations cross files and packages,
// which is the case a per-file parse cannot answer and a graph exists for.
func graphProject(t *testing.T) (string, *Project) {
	t.Helper()
	root := t.TempDir()
	write := func(name, body string) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	write("templates/card.pw.html", strings.Join([]string{
		"package widgets",
		"",
		"type Room {",
		"  id: int",
		"  title: string",
		"}",
		"",
		"export component Card(room: Room, body: html): html {",
		"  <p>{room.title}</p>",
		"}",
		"",
		"component Internal(): html {",
		"  <p>hidden</p>",
		"}",
		"",
	}, "\n"))

	write("templates/page.pw.html", strings.Join([]string{
		"package pages",
		"import \"app/widgets\"",
		"",
		"export component Page(room: Room): html {",
		"  <Card room={room} />",
		"}",
		"",
	}, "\n"))

	write("queries/rooms.pw.sql", strings.Join([]string{
		"package queries",
		"type Row { id: int }",
		"export statement RoomByID(id: int): sql.one<Row> {",
		"  SELECT id FROM rooms WHERE id = {id}",
		"}",
		"",
	}, "\n"))

	project := &Project{
		Root: root,
		Sources: []SourceDir{
			{Purpose: "generate.templates", Dir: filepath.Join(root, "templates"), Kinds: DialectsFor("generate.templates")},
			{Purpose: "generate.queries", Dir: filepath.Join(root, "queries"), Kinds: DialectsFor("generate.queries")},
		},
	}
	return root, project
}

func graphOf(t *testing.T) (string, *TypeGraph) {
	t.Helper()
	root, project := graphProject(t)
	built := buildIndex(project)
	if built.graph == nil {
		t.Fatal("the index carries no graph")
	}
	return root, built.graph
}

func TestADeclarationCarriesItsParametersAndOutput(t *testing.T) {
	root, graph := graphOf(t)

	symbol, resolved := graph.Resolve(uriOf(filepath.Join(root, "templates", "card.pw.html")), "Card")
	if !resolved {
		t.Fatal("Card does not resolve in the file that declares it")
	}
	if symbol.Kind != kindComponent || symbol.Keyword != "component" {
		t.Fatalf("symbol = %+v, want a component", symbol)
	}
	if symbol.Signature() != "export component Card(room: Room, body: html): html" {
		t.Fatalf("signature = %q", symbol.Signature())
	}
}

func TestARecordCarriesItsFields(t *testing.T) {
	root, graph := graphOf(t)

	symbol, _ := graph.Resolve(uriOf(filepath.Join(root, "templates", "card.pw.html")), "Room")
	if len(symbol.Fields) != 2 || symbol.Fields[0].Name != "id" || symbol.Fields[1].Type != "string" {
		t.Fatalf("fields = %+v", symbol.Fields)
	}
	// A record has no export modifier, so quoting one would show a word the
	// declaration does not have.
	if strings.HasPrefix(symbol.Signature(), "export ") {
		t.Fatalf("signature = %q, want no export modifier", symbol.Signature())
	}
}

func TestANameResolvesAcrossFilesInOnePackage(t *testing.T) {
	root, graph := graphOf(t)

	// Room is declared in card.pw.html and named from a file that imports the
	// package it belongs to.
	if _, resolved := graph.Resolve(uriOf(filepath.Join(root, "templates", "page.pw.html")), "Card"); !resolved {
		t.Fatal("Card does not resolve through the import")
	}
}

func TestAnUnexportedDeclarationDoesNotCrossAPackage(t *testing.T) {
	// The import makes the package's exported declarations visible and nothing
	// else, which is the rule the generator applies.
	root, graph := graphOf(t)

	if _, resolved := graph.Resolve(uriOf(filepath.Join(root, "templates", "page.pw.html")), "Internal"); resolved {
		t.Fatal("an unexported declaration crossed a package boundary")
	}
	if _, resolved := graph.Resolve(uriOf(filepath.Join(root, "templates", "card.pw.html")), "Internal"); !resolved {
		t.Fatal("an unexported declaration is not visible in its own package")
	}
}

func TestANameFromAnUnimportedPackageDoesNotResolve(t *testing.T) {
	root, graph := graphOf(t)

	if _, resolved := graph.Resolve(uriOf(filepath.Join(root, "templates", "page.pw.html")), "RoomByID"); resolved {
		t.Fatal("a declaration from an unimported package resolved")
	}
}

func TestAStatementCarriesItsResultContract(t *testing.T) {
	root, graph := graphOf(t)

	symbol, _ := graph.Resolve(uriOf(filepath.Join(root, "queries", "rooms.pw.sql")), "RoomByID")
	if symbol.Output != "sql.one<Row>" {
		t.Fatalf("output = %q, want the declared contract", symbol.Output)
	}
	if symbol.GoFunc != "RoomByID" {
		t.Fatalf("GoFunc = %q, want the generated function name", symbol.GoFunc)
	}
}

func TestALoweredGoTypeIsCarriedWhenTheAnalysisResolvedIt(t *testing.T) {
	// The one thing the parse cannot state. It is upstream's own answer, taken
	// through Signatures rather than derived here.
	root, graph := graphOf(t)

	symbol, _ := graph.Resolve(uriOf(filepath.Join(root, "templates", "card.pw.html")), "Card")
	var slot, typed bool
	for _, parameter := range symbol.Params {
		if parameter.Slot {
			slot = true
		}
		if parameter.Name == "room" && parameter.GoType != "" {
			typed = true
		}
	}
	if !typed {
		t.Fatalf("params = %+v, want a lowered Go type on room", symbol.Params)
	}
	if !slot {
		t.Fatalf("params = %+v, want the html parameter marked as a slot", symbol.Params)
	}
}

func TestAModuleThatDoesNotAnalyzeStillYieldsItsDeclarations(t *testing.T) {
	// A buffer being edited rarely analyzes cleanly. Losing the Go types is
	// acceptable; losing the declarations would make hover and navigation
	// disappear exactly while the developer is working.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Parses, and does not analyze: the component reference names nothing.
	body := "export component Page(): html {\n  <Missing />\n}\n"
	if err := os.WriteFile(filepath.Join(root, "templates", "page.pw.html"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	project := &Project{Root: root, Sources: []SourceDir{
		{Purpose: "generate.templates", Dir: filepath.Join(root, "templates"), Kinds: DialectsFor("generate.templates")},
	}}

	graph := buildIndex(project).graph
	symbol, resolved := graph.Resolve(uriOf(filepath.Join(root, "templates", "page.pw.html")), "Page")
	if !resolved {
		t.Fatal("a module that did not analyze contributed no declarations")
	}
	for _, parameter := range symbol.Params {
		if parameter.GoType != "" {
			t.Fatalf("a Go type was invented for an unanalyzed module: %+v", parameter)
		}
	}
}

func TestHoverNamesWhereADeclarationLivesAndWhatItGenerates(t *testing.T) {
	root, graph := graphOf(t)
	symbol, _ := graph.Resolve(uriOf(filepath.Join(root, "queries", "rooms.pw.sql")), "RoomByID")

	rendered := hoverFor(symbol)

	if !strings.Contains(rendered, "export statement RoomByID(id: int): sql.one<Row>") {
		t.Fatalf("hover = %q, want the signature", rendered)
	}
	if !strings.Contains(rendered, "queries/rooms.pw.sql") {
		t.Fatalf("hover = %q, want the declaring file", rendered)
	}
	if !strings.Contains(rendered, "RoomByID") || !strings.Contains(rendered, "Generated as") {
		t.Fatalf("hover = %q, want the generated function name", rendered)
	}
}

func TestTheWordUnderTheCaretIsWhatIsResolved(t *testing.T) {
	text := "export component Card(room: Room): html {\n"
	starts := newLineStarts(text)

	// Inside the word, and at either edge, because a caret sits between
	// characters and a client sends the caret.
	for _, character := range []int{17, 21, 19} {
		word, at := wordAt(text, starts, Position{Line: 0, Character: character})
		if word != "Card" {
			t.Fatalf("character %d resolved %q, want Card", character, word)
		}
		if at.Start.Character != 17 || at.End.Character != 21 {
			t.Fatalf("range = %+v, want the word", at)
		}
	}
}

func TestAPositionOnNoWordResolvesNothing(t *testing.T) {
	// Between two spaces: a caret at the edge of a word belongs to that word,
	// so only a position with no word on either side has none.
	text := "<p>  </p>\n"
	if word, _ := wordAt(text, newLineStarts(text), Position{Line: 0, Character: 4}); word != "" {
		t.Fatalf("word = %q, want none between two spaces", word)
	}
}
