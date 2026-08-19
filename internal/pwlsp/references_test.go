package pwlsp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReferencesFindEveryUseAcrossTheFilesThatCanSeeIt(t *testing.T) {
	root, project := graphProject(t)
	graph := buildIndex(project).graph
	card := uriOf(filepath.Join(root, "templates", "card.pw.html"))
	symbol, _ := graph.Resolve(card, "Card")

	found := referencesFor(referenceContext{project: project, graph: graph}, symbol, false)

	// The declaration is excluded, and the reference in the importing file is
	// not: a reference scan crosses files or it is a text search.
	if len(found) != 1 {
		t.Fatalf("references = %+v, want the one use", found)
	}
	if found[0].URI != uriOf(filepath.Join(root, "templates", "page.pw.html")) {
		t.Fatalf("uri = %s, want the importing file", found[0].URI)
	}
}

func TestTheDeclarationIsIncludedOnlyWhenAskedFor(t *testing.T) {
	root, project := graphProject(t)
	graph := buildIndex(project).graph
	symbol, _ := graph.Resolve(uriOf(filepath.Join(root, "templates", "card.pw.html")), "Card")

	with := referencesFor(referenceContext{project: project, graph: graph}, symbol, true)
	without := referencesFor(referenceContext{project: project, graph: graph}, symbol, false)

	if len(with) != len(without)+1 {
		t.Fatalf("with = %d, without = %d, want exactly the declaration between them", len(with), len(without))
	}
}

func TestAFileThatCannotSeeTheDeclarationIsNotScanned(t *testing.T) {
	// Another package's unrelated Card is a coincidence, not a reference.
	root, project := graphProject(t)
	stray := filepath.Join(root, "queries", "other.pw.sql")
	if err := os.WriteFile(stray, []byte("package queries\ntype Card { id: int }\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	graph := buildIndex(project).graph
	symbol, _ := graph.Resolve(uriOf(filepath.Join(root, "templates", "card.pw.html")), "Card")

	for _, location := range referencesFor(referenceContext{project: project, graph: graph}, symbol, true) {
		if location.URI == uriOf(stray) {
			t.Fatalf("a declaration in another package was reported as a reference: %+v", location)
		}
	}
}

func TestAnOpenBufferIsScannedRatherThanTheFile(t *testing.T) {
	root, project := graphProject(t)
	graph := buildIndex(project).graph
	page := uriOf(filepath.Join(root, "templates", "page.pw.html"))
	symbol, _ := graph.Resolve(uriOf(filepath.Join(root, "templates", "card.pw.html")), "Card")

	// The buffer has two uses where the file has one.
	open := map[string]string{
		page: "package pages\nimport \"app/widgets\"\n\nexport component Page(): html {\n  <Card />\n  <Card />\n}\n",
	}
	found := referencesFor(referenceContext{project: project, graph: graph, open: open}, symbol, false)

	if len(found) != 2 {
		t.Fatalf("references = %+v, want both uses in the buffer", found)
	}
}

func TestAWholeWordIsRequired(t *testing.T) {
	// Card must not match CardList, or a rename built on this would rewrite
	// the wrong identifier.
	text := "Card CardList myCard Card\n"
	starts := newLineStarts(text)

	found := wholeWordRanges(text, starts, "Card")

	if len(found) != 2 {
		t.Fatalf("ranges = %+v, want the two whole words", found)
	}
	if found[0].Start.Character != 0 || found[1].Start.Character != 21 {
		t.Fatalf("ranges = %+v", found)
	}
}

func TestWithNoProjectThereIsNothingToScan(t *testing.T) {
	found := referencesFor(referenceContext{}, Symbol{Name: "Card"}, true)

	if len(found) != 0 {
		t.Fatalf("references = %+v, want none", found)
	}
}
