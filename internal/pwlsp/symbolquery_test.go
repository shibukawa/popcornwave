package pwlsp

import "testing"

func TestASubstringMatchIsCaseInsensitive(t *testing.T) {
	for _, testCase := range []struct{ name, query string }{
		{"RoomByID", "room"},
		{"RoomByID", "BYID"},
		{"Card", "ar"},
	} {
		if !matches(testCase.name, testCase.query) {
			t.Errorf("%q does not match %q", testCase.name, testCase.query)
		}
	}
	if matches("Card", "zz") {
		t.Error("an unrelated query matched")
	}
}

func TestCamelHumpsMatchAnInitialism(t *testing.T) {
	// The behavior a developer already has for Go symbols in their editor.
	if !matches("PageWithLayout", "pwl") {
		t.Error("PWL does not find PageWithLayout")
	}
	if !matches("RoomByID", "rb") {
		t.Error("RB does not find RoomByID")
	}
	if matches("PageWithLayout", "pwx") {
		t.Error("a hump that is not there matched")
	}
}

func TestAnEmptyQueryMatchesEverything(t *testing.T) {
	// A client rendering the whole symbol list asks for it with an empty query.
	if !matches("anything", "") {
		t.Error("the empty query did not match")
	}
}

func TestAnOpenDocumentSupersedesItsIndexedCopy(t *testing.T) {
	// The buffer is ahead of the file whenever the developer is typing, and a
	// symbol result landing on a stale position is worse than none.
	built := index{declarations: []Declaration{
		{Name: "Card", Container: "templates/card.pw.html", URI: "file:///w/templates/card.pw.html", Kind: symbolFunction,
			Range: Range{Start: Position{Line: 9}}},
	}}
	open := []openSymbols{{
		uri:       "file:///w/templates/card.pw.html",
		container: "templates/card.pw.html",
		symbols:   []DocumentSymbol{{Name: "Card", Kind: symbolFunction, Range: Range{Start: Position{Line: 0}}}},
	}}

	found := workspaceSymbols("card", built, open)

	if len(found) != 1 {
		t.Fatalf("results = %+v, want one", found)
	}
	if found[0].Location.Range.Start.Line != 0 {
		t.Fatalf("line = %d, want the open document's position", found[0].Location.Range.Start.Line)
	}
}

func TestResultsAreOrderedByNameThenContainer(t *testing.T) {
	built := index{declarations: []Declaration{
		{Name: "Page", Container: "pages/rooms/page.pw.html", URI: "file:///w/b"},
		{Name: "Card", Container: "templates/card.pw.html", URI: "file:///w/a"},
		{Name: "Page", Container: "pages/page.pw.html", URI: "file:///w/c"},
	}}

	found := workspaceSymbols("", built, nil)

	got := []string{
		found[0].Name + " " + found[0].ContainerName,
		found[1].Name + " " + found[1].ContainerName,
		found[2].Name + " " + found[2].ContainerName,
	}
	want := []string{"Card templates/card.pw.html", "Page pages/page.pw.html", "Page pages/rooms/page.pw.html"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestAnAnswerIsBounded(t *testing.T) {
	// A client renders a list a developer scrolls, and an empty query against
	// a large project would otherwise return the whole of it.
	built := index{}
	for i := 0; i < maxSymbolResults+50; i++ {
		built.declarations = append(built.declarations, Declaration{Name: "Name", Container: "a.pw.html", URI: "file:///w/a"})
	}

	if found := workspaceSymbols("", built, nil); len(found) != maxSymbolResults {
		t.Fatalf("results = %d, want the cap of %d", len(found), maxSymbolResults)
	}
}
