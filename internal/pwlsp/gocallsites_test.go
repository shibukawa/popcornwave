package pwlsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// callSiteProject writes a project whose handwritten Go calls what a
// declaration generated, with a generated file beside the source so the names
// can be read rather than guessed.
func callSiteProject(t *testing.T) (string, *Project) {
	t.Helper()
	root := t.TempDir()
	write := func(name, body string) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("queries/rooms.pw.sql", strings.Join([]string{
		"package queries",
		"type Room { id: int }",
		"export statement RoomByID(id: int): sql.one<Room> {",
		"  SELECT id FROM rooms WHERE id = {id}",
		"}",
		"",
	}, "\n"))
	// What api:cli-generate emitted: the exported entry carrying the
	// declaration's name, and a wrapper around it.
	write("queries/rooms_pw_gen.go", strings.Join([]string{
		"package queries",
		"",
		"func RoomByID(ctx context.Context, db Querier, id int) (Room, error) { return Room{}, nil }",
		"",
		"func BuildRoomByID(id int) (Statement, error) { return Statement{}, nil }",
		"",
		"func unexported() {}",
		"",
	}, "\n"))
	write("handlers/rooms.go", strings.Join([]string{
		"package handlers",
		"",
		"func show() {",
		"\troom, _ := queries.RoomByID(ctx, db, 1)",
		"\tstatement, _ := queries.BuildRoomByID(1)",
		"\t_ = RoomByIDList",
		"\t_, _ = room, statement",
		"}",
		"",
	}, "\n"))

	project := &Project{Root: root, Sources: []SourceDir{
		{Purpose: "generate.queries", Dir: filepath.Join(root, "queries"), Kinds: DialectsFor("generate.queries")},
	}}
	return root, project
}

func TestTheGeneratedNamesAreReadRatherThanDerived(t *testing.T) {
	// The naming scheme belongs to the generator. Reproducing it here would be
	// a second copy that drifts on the next release.
	root, project := callSiteProject(t)
	graph := buildIndex(project).graph
	symbol, _ := graph.Resolve(uriOf(filepath.Join(root, "queries", "rooms.pw.sql")), "RoomByID")

	names := generatedNames(symbol)

	if len(names) != 2 || names[0] != "BuildRoomByID" || names[1] != "RoomByID" {
		t.Fatalf("names = %v, want the entry and its wrapper", names)
	}
}

func TestAnUnexportedGeneratedFunctionIsNotACallSiteName(t *testing.T) {
	root, project := callSiteProject(t)
	graph := buildIndex(project).graph
	symbol, _ := graph.Resolve(uriOf(filepath.Join(root, "queries", "rooms.pw.sql")), "RoomByID")

	for _, name := range generatedNames(symbol) {
		if name == "unexported" {
			t.Fatal("an unexported generated function was taken as a call site name")
		}
	}
}

func TestWithNoGeneratedOutputTheDeclarationNameIsUsed(t *testing.T) {
	// Navigation has to work before the first generation, which is exactly
	// when a developer is wiring a handler up.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "queries"), 0o755); err != nil {
		t.Fatal(err)
	}
	symbol := Symbol{Name: "RoomByID", GoFunc: "RoomByID", URI: uriOf(filepath.Join(root, "queries", "rooms.pw.sql"))}

	if names := generatedNames(symbol); len(names) != 1 || names[0] != "RoomByID" {
		t.Fatalf("names = %v, want the declaration name alone", names)
	}
}

func TestGoCallSitesFindTheHandwrittenCalls(t *testing.T) {
	root, project := callSiteProject(t)
	graph := buildIndex(project).graph
	symbol, _ := graph.Resolve(uriOf(filepath.Join(root, "queries", "rooms.pw.sql")), "RoomByID")

	found := goCallSites(project, symbol, nil)

	if len(found) != 2 {
		t.Fatalf("call sites = %+v, want the entry call and the wrapper call", found)
	}
	for _, location := range found {
		if !strings.HasSuffix(location.URI, "handlers/rooms.go") {
			t.Fatalf("call site in %s, want the handwritten Go", location.URI)
		}
	}
}

func TestAGeneratedFileIsNeverACallSite(t *testing.T) {
	// requirement:editor-navigation calls it a waypoint and never a
	// destination: policy:generated-artifacts makes it output, so a result
	// inside one is worthless the moment it is regenerated.
	root, project := callSiteProject(t)
	graph := buildIndex(project).graph
	symbol, _ := graph.Resolve(uriOf(filepath.Join(root, "queries", "rooms.pw.sql")), "RoomByID")

	for _, location := range goCallSites(project, symbol, nil) {
		if strings.HasSuffix(location.URI, "_pw_gen.go") {
			t.Fatalf("a generated file was reported as a call site: %s", location.URI)
		}
	}
}

func TestALongerIdentifierIsNotACallSite(t *testing.T) {
	// RoomByIDList is a different symbol, and a rename built on this would
	// rewrite it.
	root, project := callSiteProject(t)
	graph := buildIndex(project).graph
	symbol, _ := graph.Resolve(uriOf(filepath.Join(root, "queries", "rooms.pw.sql")), "RoomByID")

	source, err := os.ReadFile(filepath.Join(root, "handlers", "rooms.go"))
	if err != nil {
		t.Fatal(err)
	}
	listLine := 0
	for index, line := range strings.Split(string(source), "\n") {
		if strings.Contains(line, "RoomByIDList") {
			listLine = index
		}
	}
	for _, location := range goCallSites(project, symbol, nil) {
		if location.Range.Start.Line == listLine {
			t.Fatal("RoomByIDList was reported as a call site of RoomByID")
		}
	}
}

func TestAnOpenGoBufferIsScannedRatherThanTheFile(t *testing.T) {
	root, project := callSiteProject(t)
	graph := buildIndex(project).graph
	symbol, _ := graph.Resolve(uriOf(filepath.Join(root, "queries", "rooms.pw.sql")), "RoomByID")
	handler := uriOf(filepath.Join(root, "handlers", "rooms.go"))

	found := goCallSites(project, symbol, map[string]string{
		handler: "package handlers\n\nfunc show() { _ = RoomByID }\n",
	})

	if len(found) != 1 {
		t.Fatalf("call sites = %+v, want the one the buffer has", found)
	}
}

func TestAGeneratedSymbolResolvesBackToItsDeclaration(t *testing.T) {
	// The other direction, answered without serving a Go document: gopls owns
	// those, so this is a request a client makes rather than a provider.
	root, project := callSiteProject(t)
	graph := buildIndex(project).graph

	symbol, resolved := declarationNamed(graph, "RoomByID")
	if !resolved {
		t.Fatal("the generated entry does not resolve to its declaration")
	}
	if symbol.URI != uriOf(filepath.Join(root, "queries", "rooms.pw.sql")) {
		t.Fatalf("resolved to %s", symbol.URI)
	}

	// A wrapper carries the declaration's name inside its own, and resolves
	// only after the exact match has been tried.
	if wrapper, ok := declarationNamed(graph, "BuildRoomByID"); !ok || wrapper.Name != "RoomByID" {
		t.Fatalf("the wrapper resolved to %+v", wrapper)
	}
}

func TestAnUnrelatedGoSymbolResolvesToNothing(t *testing.T) {
	_, project := callSiteProject(t)

	if _, resolved := declarationNamed(buildIndex(project).graph, "SomethingElse"); resolved {
		t.Fatal("an unrelated symbol resolved to a declaration")
	}
}
