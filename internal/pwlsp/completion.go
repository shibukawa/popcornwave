package pwlsp

// textDocument/completion.
//
// requirement:editor-completion asks for what the position can legally hold,
// which for these dialects is a small closed set. The position is decided from
// the text around the caret rather than from the body AST, because a buffer
// mid-keystroke usually does not parse and a completion that only works on a
// parseable document is one that never appears.
//
// Every resolved item comes from the type graph. With no project loaded the
// keywords and the primitive types are still offered, and nothing that would
// need resolution is, which is the degraded answer requirement:pw-language-server
// gives everywhere else.

import (
	"strings"
)

// LSP CompletionItemKind values, named for what this server maps onto them.
const (
	itemKeyword   = 14
	itemFunction  = 3
	itemStruct    = 22
	itemEnum      = 13
	itemVariable  = 6
	itemProperty  = 10
	itemTypeParam = 25
)

// CompletionItem is one offer.
type CompletionItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind"`
	Detail string `json:"detail,omitempty"`
	// InsertText is used only where it differs from the label, which is the
	// closing form of a control block and the parameter list of a component.
	InsertText string `json:"insertText,omitempty"`
	// InsertTextFormat 2 is a snippet, which is how a placeholder is offered.
	InsertTextFormat int      `json:"insertTextFormat,omitempty"`
	Documentation    string   `json:"documentation,omitempty"`
	SortText         string   `json:"sortText,omitempty"`
	FilterText       string   `json:"filterText,omitempty"`
	AdditionalDetail string   `json:"-"`
	CommitCharacters []string `json:"commitCharacters,omitempty"`
}

// position is where the caret sits, in the terms requirement:editor-completion
// names its cases.
type completionPosition int

const (
	// posHeader is the declaration part of a file: before any body opens.
	posHeader completionPosition = iota
	// posType is a position where a type name is legal: after a colon in a
	// parameter list or a field, and inside a generic argument.
	posType
	// posExpression is inside a { } expression in a body.
	posExpression
	// posComponent is just after a < in an html body.
	posComponent
	// posBody is a body position that is none of the above.
	posBody
)

// The root keywords and modifiers a declaration can open with.
var headerKeywords = []string{
	"package", "import", "messages", "type", "enum", "external", "export",
	"component", "statement", "external async", "external live",
}

// The primitive type names every dialect accepts. A declared type comes from
// the graph; these are the ones no file declares.
var primitiveTypes = []string{
	"string", "int", "float", "bool", "datetime", "date", "time", "url", "html", "bytes",
}

// The output types each root keyword allows, per concept:template-source-dialects.
var outputTypes = map[Dialect][]string{
	dialectHTML:   {"html"},
	dialectSQL:    {"sql.one", "sql.many", "sql.exec", "sql.page"},
	dialectDynamo: {"dynamo.one", "dynamo.many", "dynamo.page", "dynamo.exec"},
}

// The control forms, offered with their closing form so a body cannot be left
// half-written by accepting a completion.
var controlForms = []struct{ label, insert, detail string }{
	{"if", "if ${1:condition}}\n\t$0\n{/if", "conditional block"},
	{"else", "else}", "the other branch of an if"},
	{"for", "for ${1:item} in ${2:items}}\n\t$0\n{/for", "loop over a collection"},
	{"await", "await ${1:value}}\n\t$0\n{fallback}\n\t\n{/await", "await block with its fallback"},
	{"val", "val ${1:name} = ${2:expression}", "bind a value for the rest of the body"},
}

// completionContext is everything an answer needs that is not the graph.
type completionContext struct {
	dialect Dialect
	uri     string
	graph   *TypeGraph
	// scope are the names bound at the caret: parameters, val bindings, and
	// loop variables, in the innermost-first order a reader expects.
	scope []Binding
}

// completionsAt answers one request.
func completionsAt(text string, starts lineStarts, at Position, context completionContext) []CompletionItem {
	offset := starts.offsetOfPosition(text, at)
	where := positionAt(text, offset)

	switch where {
	case posComponent:
		return componentItems(context)
	case posExpression:
		return expressionItems(context)
	case posType:
		return typeItems(context)
	case posHeader:
		return headerItems(context)
	default:
		return bodyItems(context)
	}
}

// positionAt decides what the caret is in, from the text before it.
//
// It reads backwards rather than parsing: the nesting that matters is one
// brace or one angle bracket deep, and the document a completion runs on is
// the one being typed into.
func positionAt(text string, offset int) completionPosition {
	before := text[:offset]

	// Inside an unclosed { }, which is an expression wherever it appears.
	if brace := strings.LastIndexByte(before, '{'); brace >= 0 {
		if !strings.ContainsAny(before[brace+1:], "}\n") {
			if isTypePosition(before[brace+1:]) {
				return posType
			}
			return posExpression
		}
	}

	// A declaration body has opened somewhere above, so a < is an element or a
	// component reference. Outside a body it is a generic argument list, which
	// is why the two are decided in this order rather than by the bracket
	// alone.
	if openBodies(before) > 0 {
		if angle := strings.LastIndexByte(before, '<'); angle >= 0 {
			if !strings.ContainsAny(before[angle+1:], "> \t\n") {
				return posComponent
			}
		}
		if isTypePosition(lastLine(before)) {
			return posType
		}
		return posBody
	}

	if isTypePosition(lastLine(before)) {
		return posType
	}
	return posHeader
}

// isTypePosition reports whether the text just before the caret puts it where
// a type name is legal: after a colon, or inside a generic argument list.
func isTypePosition(fragment string) bool {
	trimmed := strings.TrimRight(fragment, " \t")
	if strings.HasSuffix(trimmed, ":") {
		return true
	}
	if index := strings.LastIndexAny(trimmed, "<,"); index >= 0 {
		// Only inside an unclosed generic argument list.
		if strings.LastIndexByte(trimmed, '<') > strings.LastIndexByte(trimmed, '>') {
			return true
		}
		_ = index
	}
	return false
}

func lastLine(text string) string {
	if index := strings.LastIndexByte(text, '\n'); index >= 0 {
		return text[index+1:]
	}
	return text
}

// openBodies counts the declaration bodies open at the caret, by balancing the
// braces that are not part of an expression. It is deliberately crude: one
// unbalanced brace means a body is open, which is all the caller asks.
func openBodies(before string) int {
	depth := 0
	for index := 0; index < len(before); index++ {
		switch before[index] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth
}

func headerItems(context completionContext) []CompletionItem {
	items := make([]CompletionItem, 0, len(headerKeywords)+len(primitiveTypes))
	for _, keyword := range headerKeywords {
		items = append(items, CompletionItem{Label: keyword, Kind: itemKeyword, SortText: "0" + keyword})
	}
	for _, output := range outputTypes[context.dialect] {
		items = append(items, CompletionItem{
			Label: output, Kind: itemTypeParam, Detail: "output type", SortText: "1" + output,
		})
	}
	return append(items, typeItems(context)...)
}

// typeItems are the type names legal at the caret: the primitives, and every
// record and enum the file can see.
func typeItems(context completionContext) []CompletionItem {
	items := make([]CompletionItem, 0, len(primitiveTypes))
	for _, name := range primitiveTypes {
		items = append(items, CompletionItem{Label: name, Kind: itemTypeParam, Detail: "primitive", SortText: "2" + name})
	}
	for _, symbol := range visibleSymbols(context) {
		switch symbol.Kind {
		case kindType:
			items = append(items, CompletionItem{
				Label: symbol.Name, Kind: itemStruct, Detail: fieldSummary(symbol),
				Documentation: "declared in " + symbol.Container, SortText: "1" + symbol.Name,
			})
		case kindEnum:
			items = append(items, CompletionItem{
				Label: symbol.Name, Kind: itemEnum, Detail: memberSummary(symbol),
				Documentation: "declared in " + symbol.Container, SortText: "1" + symbol.Name,
			})
		}
	}
	return items
}

// componentItems are the components a body can reference, offered with the
// parameters they require so accepting one leaves a call that is complete.
func componentItems(context completionContext) []CompletionItem {
	var items []CompletionItem
	for _, symbol := range visibleSymbols(context) {
		if symbol.Kind != kindComponent {
			continue
		}
		items = append(items, CompletionItem{
			Label:            symbol.Name,
			Kind:             itemFunction,
			Detail:           symbol.Signature(),
			Documentation:    "declared in " + symbol.Container,
			InsertText:       componentSnippet(symbol),
			InsertTextFormat: 2,
		})
	}
	return items
}

// componentSnippet writes the reference with the required parameters present
// and the caret on the first of them.
func componentSnippet(symbol Symbol) string {
	var out strings.Builder
	out.WriteString(symbol.Name)
	placeholder := 1
	for _, parameter := range symbol.Params {
		if parameter.Slot {
			// A slot is filled with markup between the tags rather than by an
			// attribute, so offering it as one would be wrong.
			continue
		}
		out.WriteString(" " + parameter.Name + "=\"{$")
		out.WriteString(itoa(placeholder))
		out.WriteString("}\"")
		placeholder++
	}
	out.WriteString(" /$0>")
	return out.String()
}

// expressionItems are the names an expression can hold: what is bound at the
// caret, then every declaration the file can call.
func expressionItems(context completionContext) []CompletionItem {
	items := make([]CompletionItem, 0, len(context.scope))
	for _, binding := range context.scope {
		items = append(items, CompletionItem{
			Label: binding.Name, Kind: itemVariable, Detail: binding.Type,
			Documentation: binding.Origin, SortText: "0" + binding.Name,
		})
	}
	for _, symbol := range visibleSymbols(context) {
		if symbol.Kind != kindExternal && symbol.Kind != kindStatement {
			continue
		}
		items = append(items, CompletionItem{
			Label: symbol.Name, Kind: itemFunction, Detail: symbol.Signature(),
			Documentation: "declared in " + symbol.Container, SortText: "1" + symbol.Name,
		})
	}
	for _, form := range controlForms {
		items = append(items, CompletionItem{
			Label: form.label, Kind: itemKeyword, Detail: form.detail,
			InsertText: form.insert, InsertTextFormat: 2, SortText: "2" + form.label,
		})
	}
	return items
}

// bodyItems are what a body position that is not an expression can hold. The
// control forms are offered here too, because an author opens one by typing
// its name and the brace is what the snippet adds.
func bodyItems(context completionContext) []CompletionItem {
	items := make([]CompletionItem, 0, len(controlForms))
	for _, form := range controlForms {
		items = append(items, CompletionItem{
			Label: "{" + form.label, Kind: itemKeyword, Detail: form.detail,
			InsertText: "{" + form.insert, InsertTextFormat: 2, FilterText: form.label,
		})
	}
	return items
}

// visibleSymbols is what the file can name, or nothing with no project.
func visibleSymbols(context completionContext) []Symbol {
	if context.graph == nil {
		return nil
	}
	return context.graph.Visible(context.uri)
}

func fieldSummary(symbol Symbol) string {
	names := make([]string, 0, len(symbol.Fields))
	for _, field := range symbol.Fields {
		names = append(names, field.Name)
	}
	return "record {" + strings.Join(names, ", ") + "}"
}

func memberSummary(symbol Symbol) string {
	names := make([]string, 0, len(symbol.Fields))
	for _, field := range symbol.Fields {
		names = append(names, field.Name)
	}
	return "enum " + strings.Join(names, " | ")
}

func itoa(value int) string {
	if value < 10 {
		return string(rune('0' + value))
	}
	return strings.TrimSpace(strings.Join([]string{itoa(value / 10), itoa(value % 10)}, ""))
}
