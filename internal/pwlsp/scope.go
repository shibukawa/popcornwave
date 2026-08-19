package pwlsp

// What is bound at a position in a body.
//
// A parameter is written in the header and a val binding, a loop variable, and
// an await binding are written in the body, so the names an expression may hold
// are a property of where the caret is rather than of the file. This is the
// layer requirement:editor-inlay-hints and the expression half of
// requirement:editor-completion both read.
//
// The walk is over the parse, and it descends through the html element tree as
// well as the shared control nodes, because a {for} inside a <ul> is nested in
// the element rather than beside it.

import (
	"github.com/shibukawa/tinybind-go/templates/htmlbind"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// Binding is one name in scope.
type Binding struct {
	Name string
	// Type is the resolved type where the source states it, and empty where it
	// does not. A val binding's type is the result of an expression, and this
	// server does not evaluate expressions, so it is filled only when the
	// binding names a declaration whose output the graph knows.
	Type string
	// Origin says where the name came from, in the words a reader would use.
	Origin string
	// Pos is where the name is written, used to place an inlay hint after it.
	Pos sqlbind.Position
	// Kind distinguishes the sources so a hint can be switched off per family.
	Kind BindingKind
}

// BindingKind names the families requirement:editor-inlay-hints lets a
// developer switch on and off separately.
type BindingKind string

const (
	bindingParameter BindingKind = "parameter"
	bindingVal       BindingKind = "binding"
	bindingLoop      BindingKind = "loop"
	bindingAwait     BindingKind = "await"
)

// bindingsIn collects every binding of one declaration, with the enclosing
// declaration's parameters first.
//
// Scoping is not narrowed to the caret: a val binding scopes its later
// siblings, a loop variable scopes its own body, and deciding which of those
// contains a byte offset needs an end position the parser does not record. A
// name offered slightly too widely costs a completion entry; narrowing it
// wrongly would hide the one the author is typing.
func bindingsIn(declaration *sqlbind.TemplateDecl, graph *TypeGraph, uri string) []Binding {
	bindings := make([]Binding, 0, len(declaration.Parameters))
	for _, parameter := range declaration.Parameters {
		bindings = append(bindings, Binding{
			Name:   parameter.Name,
			Type:   typeName(parameter.Type),
			Origin: "a parameter of " + declaration.Name,
			Pos:    parameter.Pos,
			Kind:   bindingParameter,
		})
	}
	nodes, ok := declaration.Body.([]sqlbind.Node)
	if !ok {
		return bindings
	}
	return append(bindings, walkBindings(nodes, graph, uri)...)
}

func walkBindings(nodes []sqlbind.Node, graph *TypeGraph, uri string) []Binding {
	var bindings []Binding
	for _, node := range nodes {
		switch typed := node.(type) {
		case *sqlbind.ValNode:
			for _, binding := range typed.Bindings {
				bindings = append(bindings, Binding{
					Name:   binding.Name,
					Type:   expressionType(binding.Value, graph, uri),
					Origin: "a val binding",
					Pos:    binding.Pos,
					Kind:   bindingVal,
				})
			}
			bindings = append(bindings, walkBindings(typed.Body, graph, uri)...)
		case *sqlbind.ForNode:
			bindings = append(bindings, Binding{
				Name:   typed.Variable,
				Type:   elementType(typed.Iterable, graph, uri),
				Origin: "the loop variable",
				Pos:    typed.Pos,
				Kind:   bindingLoop,
			})
			if typed.Index != "" {
				bindings = append(bindings, Binding{
					Name: typed.Index, Type: "int", Origin: "the loop index", Pos: typed.Pos, Kind: bindingLoop,
				})
			}
			bindings = append(bindings, walkBindings(typed.Body, graph, uri)...)
		case *sqlbind.IfNode:
			bindings = append(bindings, walkBindings(typed.Then, graph, uri)...)
			bindings = append(bindings, walkBindings(typed.Else, graph, uri)...)
		case *htmlbind.AwaitNode:
			for _, binding := range typed.Bindings {
				bindings = append(bindings, Binding{
					Name:   binding.Name,
					Type:   expressionType(binding.Call, graph, uri),
					Origin: "an await binding",
					Pos:    binding.Pos,
					Kind:   bindingAwait,
				})
			}
			bindings = append(bindings, walkBindings(typed.Primary, graph, uri)...)
			bindings = append(bindings, walkBindings(typed.Fallback, graph, uri)...)
			bindings = append(bindings, walkBindings(typed.Recover, graph, uri)...)
		case *htmlbind.ElementNode:
			bindings = append(bindings, walkBindings(typed.Children, graph, uri)...)
		case *htmlbind.ComponentNode:
			bindings = append(bindings, walkBindings(typed.Children, graph, uri)...)
		case *htmlbind.HeadNode:
			bindings = append(bindings, walkBindings(typed.Children, graph, uri)...)
		case *htmlbind.SlotNode:
			bindings = append(bindings, walkBindings(typed.Default, graph, uri)...)
		}
	}
	return bindings
}

// expressionType is the type an expression yields, where the graph can say so
// without evaluating anything.
//
// A call of a declaration yields that declaration's output; everything else
// yields nothing rather than a guess. That covers the case the hint exists
// for — a val binding of a query or a component call, where the source never
// writes the type — and refuses the rest honestly.
func expressionType(expression sqlbind.Expr, graph *TypeGraph, uri string) string {
	call, ok := expression.(*sqlbind.CallExpr)
	if !ok || graph == nil {
		return ""
	}
	name, named := calleeName(call)
	if !named {
		return ""
	}
	symbol, resolved := graph.Resolve(uri, name)
	if !resolved {
		return ""
	}
	return symbol.Output
}

// elementType is what one iteration of a collection yields.
//
// The collection is usually a parameter or a binding rather than a call, and
// resolving that needs the type of a name in scope, which is the one thing a
// flat binding list cannot answer without ordering. Only the call case is
// answered, and the array suffix is stripped because a loop binds an element.
func elementType(iterable sqlbind.Expr, graph *TypeGraph, uri string) string {
	rendered := expressionType(iterable, graph, uri)
	if rendered == "" {
		return ""
	}
	if len(rendered) > 2 && rendered[len(rendered)-2:] == "[]" {
		return rendered[:len(rendered)-2]
	}
	return rendered
}

// calleeName is the name a call expression invokes, when it is a plain
// identifier. A method call on a value is not a declaration reference.
func calleeName(call *sqlbind.CallExpr) (string, bool) {
	identifier, ok := call.Callee.(*sqlbind.IdentifierExpr)
	if !ok {
		return "", false
	}
	return identifier.Name, true
}
