package pwlsp

import (
	"strings"
	"testing"
)

func labels(items []CompletionItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Label)
	}
	return out
}

func has(items []CompletionItem, label string) bool {
	for _, item := range items {
		if item.Label == label {
			return true
		}
	}
	return false
}

func itemNamed(items []CompletionItem, label string) (CompletionItem, bool) {
	for _, item := range items {
		if item.Label == label {
			return item, true
		}
	}
	return CompletionItem{}, false
}

func TestThePositionDecidesWhatIsOffered(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		before string
		want   completionPosition
	}{
		{"the top of a file", "package p\n", posHeader},
		{"after a parameter colon", "export component C(room: ", posType},
		{"inside a generic argument", "export statement S(): sql.one<", posType},
		{"after an opening angle in a body", "export component C(): html {\n  <", posComponent},
		{"inside an expression", "export component C(): html {\n  <p>{", posExpression},
		{"a plain body position", "export component C(): html {\n  <p>x</p>\n  ", posBody},
	} {
		if got := positionAt(testCase.before, len(testCase.before)); got != testCase.want {
			t.Errorf("%s: position = %v, want %v", testCase.name, got, testCase.want)
		}
	}
}

func TestAHeaderOffersTheRootKeywordsAndTheOutputTypesOfItsDialect(t *testing.T) {
	items := completionsAt("", newLineStarts(""), Position{}, completionContext{dialect: dialectSQL})

	if !has(items, "export") || !has(items, "statement") {
		t.Fatalf("items = %v, want the root keywords", labels(items))
	}
	if !has(items, "sql.one") {
		t.Fatalf("items = %v, want the SQL output types", labels(items))
	}
	// A dialect's outputs are its own: offering html here would offer a
	// contract the root keyword does not allow.
	if has(items, "dynamo.many") {
		t.Fatalf("items = %v, want no dynamo outputs in a SQL file", labels(items))
	}
}

func TestATypePositionOffersThePrimitivesAndTheDeclaredRecords(t *testing.T) {
	root, graph := graphOf(t)
	uri := uriOf(root + "/templates/card.pw.html")
	text := "package widgets\nexport component C(room: "

	items := completionsAt(text, newLineStarts(text), Position{Line: 1, Character: 25}, completionContext{
		dialect: dialectHTML, uri: uri, graph: graph,
	})

	if !has(items, "string") || !has(items, "int") {
		t.Fatalf("items = %v, want the primitives", labels(items))
	}
	if !has(items, "Room") {
		t.Fatalf("items = %v, want the record the file declares", labels(items))
	}
	// A component is not a type, so it does not belong in a type position.
	if has(items, "Card") {
		t.Fatalf("items = %v, want no component in a type position", labels(items))
	}
}

func TestAComponentPositionOffersItsParameters(t *testing.T) {
	// Accepting a completion must leave a call that is complete, per
	// requirement:editor-completion.
	root, graph := graphOf(t)
	uri := uriOf(root + "/templates/card.pw.html")
	text := "package widgets\nexport component X(): html {\n  <"

	items := completionsAt(text, newLineStarts(text), Position{Line: 2, Character: 3}, completionContext{
		dialect: dialectHTML, uri: uri, graph: graph,
	})

	card, offered := itemNamed(items, "Card")
	if !offered {
		t.Fatalf("items = %v, want the component", labels(items))
	}
	if !strings.Contains(card.InsertText, "room=") {
		t.Fatalf("insert = %q, want the required parameter", card.InsertText)
	}
	// An html parameter is a slot, filled between the tags rather than by an
	// attribute, so offering it as one would be wrong.
	if strings.Contains(card.InsertText, "body=") {
		t.Fatalf("insert = %q, want no attribute for the slot", card.InsertText)
	}
}

func TestAnExpressionOffersWhatIsBoundAtIt(t *testing.T) {
	root, graph := graphOf(t)
	uri := uriOf(root + "/templates/card.pw.html")
	text := "package widgets\nexport component C(room: Room): html {\n  <p>{"

	items := completionsAt(text, newLineStarts(text), Position{Line: 2, Character: 6}, completionContext{
		dialect: dialectHTML, uri: uri, graph: graph,
		scope: []Binding{{Name: "room", Type: "Room", Origin: "a parameter of C", Kind: bindingParameter}},
	})

	if !has(items, "room") {
		t.Fatalf("items = %v, want the parameter in scope", labels(items))
	}
	// A control form is offered with its closing form, so accepting one cannot
	// leave a body half-written.
	form, offered := itemNamed(items, "for")
	if !offered || !strings.Contains(form.InsertText, "{/for") {
		t.Fatalf("for = %+v, want the closing form in the snippet", form)
	}
}

func TestWithNoProjectOnlyWhatNeedsNoResolutionIsOffered(t *testing.T) {
	// The degraded answer requirement:editor-completion states: snippets and
	// keywords, and nothing that would need the project.
	text := "export component C(room: "

	items := completionsAt(text, newLineStarts(text), Position{Line: 0, Character: len(text)}, completionContext{
		dialect: dialectHTML,
	})

	if !has(items, "string") {
		t.Fatalf("items = %v, want the primitives", labels(items))
	}
	for _, item := range items {
		if item.Kind == itemStruct || item.Kind == itemFunction {
			t.Fatalf("items = %v, want nothing that needed resolution", labels(items))
		}
	}
}

func TestABodyOffersTheControlFormsWithTheirBrace(t *testing.T) {
	text := "export component C(): html {\n  <p>x</p>\n  "

	items := completionsAt(text, newLineStarts(text), Position{Line: 2, Character: 2}, completionContext{
		dialect: dialectHTML,
	})

	form, offered := itemNamed(items, "{if")
	if !offered {
		t.Fatalf("items = %v, want the control forms", labels(items))
	}
	if form.FilterText != "if" {
		t.Fatalf("filter = %q, want the bare word so typing if matches", form.FilterText)
	}
}
