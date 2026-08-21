package pwlsp

// textDocument/documentSymbol over a parsed module.
//
// The outline is the declaration list, not the body: a reader opening the
// outline of a .pw.html wants the components it declares, and a body node is
// markup they can already see. requirement:editor-navigation is what will
// resolve a name; this only lists what the file itself declares.

import (
	"strings"

	"github.com/shibukawa/tinybind-go/templates/dynamobind"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// LSP SymbolKind values, named for what this server maps onto them rather
// than for the whole enumeration.
const (
	symbolFunction = 12
	symbolStruct   = 23
	symbolEnum     = 10
	symbolField    = 8
)

// DocumentSymbol is the hierarchical shape of the response. Range covers the
// whole declaration and SelectionRange covers the name a client reveals; both
// are the declaration position here, because the parsers report where a
// declaration starts and not where it ends.
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

func documentSymbols(found analysis, source string, starts lineStarts) []DocumentSymbol {
	symbols := []DocumentSymbol{}
	for _, declaration := range moduleDeclarations(found.module) {
		symbols = append(symbols, declarationSymbol(declaration, source, starts))
	}
	for _, query := range found.dynamo {
		symbols = append(symbols, dynamoSymbol(query, source, starts))
	}
	return symbols
}

func moduleDeclarations(module *sqlbind.Module) []sqlbind.Declaration {
	if module == nil {
		return nil
	}
	return module.Declarations
}

func declarationSymbol(declaration sqlbind.Declaration, source string, starts lineStarts) DocumentSymbol {
	switch node := declaration.(type) {
	case *sqlbind.TemplateDecl:
		return DocumentSymbol{
			Name:           node.Name,
			Detail:         templateDetail(node),
			Kind:           symbolFunction,
			Range:          nameRange(node.Pos.Line, node.Pos.Col, source, starts),
			SelectionRange: nameRange(node.Pos.Line, node.Pos.Col, source, starts),
		}
	case *sqlbind.TypeDecl:
		fields := make([]DocumentSymbol, 0, len(node.Fields))
		for _, field := range node.Fields {
			fields = append(fields, DocumentSymbol{
				Name:           field.Name,
				Detail:         typeName(field.Type),
				Kind:           symbolField,
				Range:          nameRange(field.Pos.Line, field.Pos.Col, source, starts),
				SelectionRange: nameRange(field.Pos.Line, field.Pos.Col, source, starts),
			})
		}
		return DocumentSymbol{
			Name:           node.Name,
			Kind:           symbolStruct,
			Range:          nameRange(node.Pos.Line, node.Pos.Col, source, starts),
			SelectionRange: nameRange(node.Pos.Line, node.Pos.Col, source, starts),
			Children:       fields,
		}
	case *sqlbind.EnumDecl:
		members := make([]DocumentSymbol, 0, len(node.Members))
		for _, member := range node.Members {
			members = append(members, DocumentSymbol{
				Name:           member.Name,
				Kind:           symbolField,
				Range:          nameRange(member.Pos.Line, member.Pos.Col, source, starts),
				SelectionRange: nameRange(member.Pos.Line, member.Pos.Col, source, starts),
			})
		}
		return DocumentSymbol{
			Name:           node.Name,
			Kind:           symbolEnum,
			Range:          nameRange(node.Pos.Line, node.Pos.Col, source, starts),
			SelectionRange: nameRange(node.Pos.Line, node.Pos.Col, source, starts),
			Children:       members,
		}
	case *sqlbind.ExternalDecl:
		return DocumentSymbol{
			Name:           node.Name,
			Detail:         externalDetail(node),
			Kind:           symbolFunction,
			Range:          nameRange(node.Pos.Line, node.Pos.Col, source, starts),
			SelectionRange: nameRange(node.Pos.Line, node.Pos.Col, source, starts),
		}
	default:
		return DocumentSymbol{Name: "?", Kind: symbolFunction}
	}
}

// dynamoSymbol lists an access pattern. The dynamo parser reports a line and
// no column, so the symbol starts at the beginning of that line.
func dynamoSymbol(query dynamobind.QueryDecl, source string, starts lineStarts) DocumentSymbol {
	detail := "dynamo." + string(query.Shape)
	if query.ItemType != "" {
		detail += "<" + query.ItemType + ">"
	}
	if query.Table != "" {
		detail += " on " + query.Table
	}
	return DocumentSymbol{
		Name:           query.Name,
		Detail:         detail,
		Kind:           symbolFunction,
		Range:          nameRange(query.Line, 1, source, starts),
		SelectionRange: nameRange(query.Line, 1, source, starts),
	}
}

func templateDetail(node *sqlbind.TemplateDecl) string {
	detail := rootKeyword(node.Kind) + " " + parameterList(node.Parameters)
	if output := typeName(node.Output); output != "" {
		detail += ": " + output
	}
	return detail
}

func externalDetail(node *sqlbind.ExternalDecl) string {
	prefix := "external"
	if node.Async {
		prefix += " async"
	}
	if node.Live {
		prefix += " live"
	}
	detail := prefix + " " + parameterList(node.Parameters)
	if result := typeName(node.Result); result != "" {
		detail += ": " + result
	}
	return detail
}

func parameterList(parameters []sqlbind.Parameter) string {
	names := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		names = append(names, parameter.Name+": "+typeName(parameter.Type))
	}
	return "(" + strings.Join(names, ", ") + ")"
}

func nameRange(line, column int, source string, starts lineStarts) Range {
	return starts.rangeAt(source, starts.offsetOf(source, line, column))
}

// declaredNameRange is where a declaration writes its own name.
//
// The parser records where the declaration starts, which is its keyword or its
// export modifier. That is the right range for an outline entry and the wrong
// one for a jump: go-to-definition should land on the name, and a reference
// scan needs the name's position to leave the declaration out of its own
// results. The name is found forward from the declaration, as a whole word.
func declaredNameRange(line, column int, name, source string, starts lineStarts) Range {
	from := starts.offsetOf(source, line, column)
	// One forward scan to the first whole-word occurrence, against the
	// document's own line table. The previous version collected every match in
	// the rest of the document — with a fresh line table per declaration — to
	// use only the first, and converted it back through a table built for the
	// slice rather than the document, which was only right for a name on the
	// declaration's own line.
	for offset := from; name != ""; {
		index := strings.Index(source[offset:], name)
		if index < 0 {
			break
		}
		start := offset + index
		end := start + len(name)
		offset = end
		if start > from && isWordByte(source[start-1]) {
			continue
		}
		if end < len(source) && isWordByte(source[end]) {
			continue
		}
		return starts.rangeAt(source, start)
	}
	return starts.rangeAt(source, from)
}

// typeName renders a TypeRef the way a declaration spells it, so the outline
// detail reads as the source does rather than as the AST does. The parser
// keeps the modifiers as flags, and this is the only place that has to put
// them back in order.
func typeName(reference sqlbind.TypeRef) string {
	if reference.Name == "" {
		return ""
	}
	name := reference.Name
	if len(reference.Arguments) > 0 {
		arguments := make([]string, 0, len(reference.Arguments))
		for _, argument := range reference.Arguments {
			arguments = append(arguments, typeName(argument))
		}
		name += "<" + strings.Join(arguments, ", ") + ">"
	}
	if reference.Array {
		name += "[]"
	}
	if reference.Optional {
		name += "?"
	}
	if reference.Async {
		name = "async " + name
	}
	return name
}
