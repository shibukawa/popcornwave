package pwlsp

// textDocument/hover and textDocument/definition over the type graph.
//
// Both answer about the identifier under the cursor. That is name resolution
// rather than a guess: these dialects have one namespace per package, a local
// declaration shadows an import, and a name written in a body means the
// declaration of that name or nothing. What it does not do is decide whether
// the identifier was written in a position where a reference is legal — inside
// a SQL string literal, a name that matches a declaration still resolves — and
// that is the honest limit of answering from the graph rather than from the
// body AST.

import (
	"strings"
)

// Hover is the reply to textDocument/hover. Markdown, because a signature
// reads as code and a client renders a fenced block.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// wordAt returns the identifier the position falls in, and its range.
//
// A position at either edge of a word belongs to it, because a client sends
// the caret and a caret sits between characters.
func wordAt(text string, starts lineStarts, at Position) (string, Range) {
	offset := starts.offsetOfPosition(text, at)
	start, end := offset, offset
	for start > 0 && isWordByte(text[start-1]) {
		start--
	}
	for end < len(text) && isWordByte(text[end]) {
		end++
	}
	if start == end {
		return "", Range{}
	}
	return text[start:end], Range{Start: starts.positionOf(text, start), End: starts.positionOf(text, end)}
}

// hoverFor renders what is known about a name.
//
// The signature first, because that is the question; then where it is
// declared, because a name resolved across files is exactly the case where
// that is not obvious; then the generated Go function, which is the name a
// handwritten call site uses and the one thing the source never states.
func hoverFor(symbol Symbol) string {
	var out strings.Builder
	out.WriteString("```pw\n")
	out.WriteString(symbol.Signature())
	out.WriteString("\n```\n")

	if len(symbol.Fields) > 0 {
		out.WriteString("\n")
		for _, field := range symbol.Fields {
			out.WriteString("- `" + field.Name + "`")
			if field.Type != "" {
				out.WriteString(": " + field.Type)
			}
			out.WriteString("\n")
		}
	}

	lowered := loweredParams(symbol)
	if len(lowered) > 0 {
		out.WriteString("\nGenerated parameters:\n")
		for _, line := range lowered {
			out.WriteString("- " + line + "\n")
		}
	}

	out.WriteString("\nDeclared in `" + symbol.Container + "`")
	if symbol.Package != "" {
		out.WriteString(", package `" + symbol.Package + "`")
	}
	out.WriteString(".")
	if symbol.GoFunc != "" {
		out.WriteString("\n\nGenerated as `" + symbol.GoFunc + "` in the package beside it.")
	}
	return out.String()
}

// loweredParams are the parameters whose Go type the analysis resolved. An
// unanalyzed module contributes none, which is why they are a separate section
// rather than part of the signature: a hover that silently dropped the types
// would read as a declaration with no parameters.
func loweredParams(symbol Symbol) []string {
	var lines []string
	for _, parameter := range symbol.Params {
		if parameter.GoType == "" {
			continue
		}
		line := "`" + parameter.Name + "` " + parameter.GoType
		if parameter.Slot {
			line += " (a slot, filled with markup)"
		}
		lines = append(lines, line)
	}
	return lines
}
