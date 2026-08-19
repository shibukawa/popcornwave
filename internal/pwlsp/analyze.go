package pwlsp

// Syntax analysis for one open document.
//
// The parsers are system:tinybind's own, per decision:language-server-in-pw-cli:
// a diagnostic reported here is the diagnostic api:cli-generate would report,
// because it comes from the same parse. Nothing in this file resolves a type
// or reads a second file, which is what makes it the parse-only mode
// requirement:pw-language-server serves with no project.

import (
	"errors"
	"strconv"
	"strings"

	"github.com/shibukawa/popcornweb/internal/pwgen"
	"github.com/shibukawa/tinybind-go/templates/dynamobind"
	"github.com/shibukawa/tinybind-go/templates/htmlbind"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// Dialect is which of concept:template-source-dialects a document is written
// in. It is decided by the file name alone, exactly as api:cli-generate decides
// it, so an editor and a generation never disagree about a file. It is exported
// only because the project model's source list is built by the caller that
// reads popcornweb.toml.
type Dialect string

const (
	dialectHTML   Dialect = "html"
	dialectSQL    Dialect = "sql"
	dialectDynamo Dialect = "dynamo"
	dialectNone   Dialect = ""
)

// suffixes are the Popcorn Web names of the three dialects. They are the
// pwgen patterns with the glob removed, so a rename upstream reaches here.
var suffixes = map[Dialect]string{
	dialectHTML:   strings.TrimPrefix(pwgen.HTMLTemplatePattern, "*"),
	dialectSQL:    strings.TrimPrefix(pwgen.SQLTemplatePattern, "*"),
	dialectDynamo: strings.TrimPrefix(pwgen.DynamoTemplatePattern, "*"),
}

func dialectOf(name string) Dialect {
	for kind, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			return kind
		}
	}
	return dialectNone
}

// severity values LSP defines, mapped to the ranks
// requirement:editor-diagnostics fixes: error is what would stop
// api:cli-generate, warning is what it warns about and carries on from.
const (
	severityError   = 1
	severityWarning = 2
)

// Diagnostic is one finding, in the shape textDocument/publishDiagnostics
// carries. Source names this server so a client can tell a pw finding from a
// gopls one on the same file.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

// analysis is what one parse produced: the module when the source parsed, and
// the diagnostics either way. htmlbind.Module and sqlbind.Module are two names
// for one shared type, so the two dialects with a module share this field.
type analysis struct {
	module      *sqlbind.Module
	dynamo      []dynamobind.QueryDecl
	diagnostics []Diagnostic
}

// analyze parses one document and reports what the parser found. A parse
// failure is a diagnostic rather than an error, because a document being
// edited is expected to be unparseable most of the time.
func analyze(name string, kind Dialect, source string, starts lineStarts) analysis {
	var err error
	out := analysis{diagnostics: []Diagnostic{}}
	switch kind {
	case dialectHTML:
		var module *htmlbind.Module
		module, err = htmlbind.Parse(name, []byte(source))
		out.module = module
	case dialectSQL:
		out.module, err = sqlbind.Parse(name, []byte(source))
	case dialectDynamo:
		out.dynamo, err = dynamobind.ParseQueries(name, []byte(source))
	default:
		return out
	}
	if err != nil {
		out.module, out.dynamo = nil, nil
		out.diagnostics = append(out.diagnostics, diagnosticFor(err, source, starts))
	}
	return out
}

// diagnosticFor places a parser error in the document.
//
// The structured error carries a line, a column, and the file name it was
// reported against; the dynamo parser reports a formatted "file:line: message"
// instead, so the text is read when the structure is absent. Neither path
// invents a position: a message with none is reported at the start of the
// file, where a reader will still see it.
func diagnosticFor(err error, source string, starts lineStarts) Diagnostic {
	message := err.Error()
	line, column := 0, 1

	var parsed *sqlbind.ParseError
	if errors.As(err, &parsed) {
		line, column, message = parsed.Line, parsed.Column, parsed.Message
	} else if position, rest, found := parsePrefixedPosition(message); found {
		line, column, message = position.line, position.column, rest
	}

	diagnostic := Diagnostic{Severity: severityError, Source: "pw", Message: message}
	if line < 1 {
		diagnostic.Range = Range{}
		return diagnostic
	}
	diagnostic.Range = starts.rangeAt(source, starts.offsetOf(source, line, column))
	return diagnostic
}

type textPosition struct{ line, column int }

// parsePrefixedPosition reads the "<name>:<line>[:<column>]: " prefix that a
// parser without a structured error writes, and returns the message with the
// prefix removed. The scan takes the first colon followed by digits and
// another colon, so a file name holding a colon of its own is not mistaken for
// a position.
func parsePrefixedPosition(message string) (textPosition, string, bool) {
	for index := 0; index < len(message); index++ {
		if message[index] != ':' {
			continue
		}
		end := index + 1 + digitRun(message[index+1:])
		if end == index+1 || end >= len(message) || message[end] != ':' {
			continue
		}
		line, err := strconv.Atoi(message[index+1 : end])
		if err != nil {
			continue
		}
		position := textPosition{line: line, column: 1}
		rest := message[end+1:]
		if run := digitRun(rest); run > 0 && run < len(rest) && rest[run] == ':' {
			position.column, _ = strconv.Atoi(rest[:run])
			rest = rest[run+1:]
		}
		return position, strings.TrimSpace(rest), true
	}
	return textPosition{}, message, false
}

// digitRun is the length of the leading run of digits in text.
func digitRun(text string) int {
	index := 0
	for index < len(text) && text[index] >= '0' && text[index] <= '9' {
		index++
	}
	return index
}
