package pwlsp

import (
	"errors"
	"testing"

	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

func TestDialectFollowsTheFileName(t *testing.T) {
	// The suffixes come from pwgen, so the editor and api:cli-generate decide
	// a file's dialect the same way rather than from two lists.
	for name, want := range map[string]Dialect{
		"page.pw.html":            dialectHTML,
		"queries/accounts.pw.sql": dialectSQL,
		"readings.pw.dynamo":      dialectDynamo,
		"page.html":               dialectNone,
		"main.go":                 dialectNone,
		"notes.pw.txt":            dialectNone,
	} {
		if got := dialectOf(name); got != want {
			t.Errorf("dialectOf(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestACleanSourceReportsNothing(t *testing.T) {
	source := "package q\ntype R { id: int }\nexport statement F(id: int): sql.one<R> {\n  SELECT id FROM t WHERE id = {id}\n}\n"

	found := analyze("q.pw.sql", dialectSQL, source, newLineStarts(source))

	if len(found.diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none", found.diagnostics)
	}
	if found.module == nil {
		t.Fatal("a source that parsed produced no module")
	}
}

func TestAnHTMLSyntaxErrorIsPlacedInTheDocument(t *testing.T) {
	source := "export component X(): html {\n  <p>unclosed\n}\n"

	found := analyze("x.pw.html", dialectHTML, source, newLineStarts(source))

	if len(found.diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want one", found.diagnostics)
	}
	if found.module != nil {
		t.Fatal("a failed parse produced a module")
	}
	diagnostic := found.diagnostics[0]
	if diagnostic.Severity != severityError || diagnostic.Source != "pw" {
		t.Fatalf("diagnostic = %+v", diagnostic)
	}
	// The message is the parser's own, with the position stripped out of it:
	// the position is carried by the range, and a client showing both would
	// print the file name twice.
	if diagnostic.Message == "" || containsPosition(diagnostic.Message) {
		t.Fatalf("message = %q, want the text with no position prefix", diagnostic.Message)
	}
}

func TestADynamoSyntaxErrorIsPlacedFromItsMessage(t *testing.T) {
	// The dynamo parser reports "file:line: message" rather than a structured
	// error, so this is the path that reads a position out of the text.
	source := "export statement Q(k: string): dynamo.many<R> {\n  key pk = {k}\n}\n"

	found := analyze("r.pw.dynamo", dialectDynamo, source, newLineStarts(source))

	if len(found.diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want one", found.diagnostics)
	}
	if containsPosition(found.diagnostics[0].Message) {
		t.Fatalf("message = %q, want the text with no position prefix", found.diagnostics[0].Message)
	}
}

func TestAnUnknownDialectIsNotParsed(t *testing.T) {
	found := analyze("notes.txt", dialectNone, "anything at all", newLineStarts("anything at all"))

	if len(found.diagnostics) != 0 || found.module != nil {
		t.Fatalf("analysis = %+v, want an empty one", found)
	}
}

func TestAStructuredErrorPositionWins(t *testing.T) {
	source := "one\ntwo\nthree\n"
	starts := newLineStarts(source)

	diagnostic := diagnosticFor(&sqlbind.ParseError{Filename: "q.pw.sql", Line: 2, Column: 2, Message: "bad"}, source, starts)

	if diagnostic.Range.Start.Line != 1 || diagnostic.Range.Start.Character != 1 {
		t.Fatalf("range = %+v, want line 2 column 2 as a zero-based position", diagnostic.Range)
	}
	if diagnostic.Message != "bad" {
		t.Fatalf("message = %q", diagnostic.Message)
	}
}

func TestAnErrorWithNoPositionLandsAtTheStartOfTheFile(t *testing.T) {
	source := "one\n"

	diagnostic := diagnosticFor(errors.New("something went wrong"), source, newLineStarts(source))

	if diagnostic.Range != (Range{}) {
		t.Fatalf("range = %+v, want the start of the file", diagnostic.Range)
	}
	if diagnostic.Message != "something went wrong" {
		t.Fatalf("message = %q", diagnostic.Message)
	}
}

func TestPrefixedPositionsAreReadOutOfAMessage(t *testing.T) {
	for _, testCase := range []struct {
		message string
		line    int
		column  int
		rest    string
		found   bool
	}{
		{"page.pw.html:12:3: bad thing", 12, 3, "bad thing", true},
		{"readings.pw.dynamo:7: bad thing", 7, 1, "bad thing", true},
		{`C:\work\page.pw.html:4:1: bad thing`, 4, 1, "bad thing", true},
		{"no position here", 0, 0, "no position here", false},
		{"ratio 3:4 is not a position", 0, 0, "ratio 3:4 is not a position", false},
	} {
		position, rest, found := parsePrefixedPosition(testCase.message)
		if found != testCase.found || rest != testCase.rest {
			t.Errorf("%q -> (%v, %q), want (%v, %q)", testCase.message, found, rest, testCase.found, testCase.rest)
			continue
		}
		if found && (position.line != testCase.line || position.column != testCase.column) {
			t.Errorf("%q -> %d:%d, want %d:%d", testCase.message, position.line, position.column, testCase.line, testCase.column)
		}
	}
}

func containsPosition(message string) bool {
	_, _, found := parsePrefixedPosition(message)
	return found
}
