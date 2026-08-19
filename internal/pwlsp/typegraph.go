package pwlsp

// The resolved name graph stage 3 answers from.
//
// It is built from the parse rather than from a second analysis, because these
// dialects declare their types: a parameter, a record field, and an output are
// all written in the source, so resolving a name is a lookup rather than an
// inference. What the parse cannot state is the Go a declaration lowers to, and
// that comes from system:tinybind's own Signatures where the module analyzes
// cleanly, per the first principle of vision:editor-support.
//
// Nothing here reaches a Go package. requirement:editor-navigation's Go
// directions need the generated output and the call sites gopls already
// indexes; this is the template-to-template half, which is the half that works
// with no generated output at all.

import (
	"path/filepath"
	"strings"

	"github.com/shibukawa/tinybind-go/templates/htmlbind"
	"github.com/shibukawa/tinybind-go/templates/sqlbind"
)

// Param is one declared parameter, as the source spells it.
type Param struct {
	Name string
	Type string
	// GoType is the parameter's lowered Go type, when the module analyzed
	// cleanly. Empty means the analysis did not run or did not finish, which is
	// the ordinary state of a buffer being edited.
	GoType string
	// Slot marks an html parameter, which a caller fills with markup rather
	// than with data.
	Slot bool
}

// Field is one record field or enum member.
type Field struct {
	Name string
	Type string
	Line int
}

// SymbolKind is what a name declares. It is coarser than the LSP symbol kinds
// because a hover reads it as a word.
type SymbolKind string

const (
	kindComponent SymbolKind = "component"
	kindStatement SymbolKind = "statement"
	kindType      SymbolKind = "type"
	kindEnum      SymbolKind = "enum"
	kindExternal  SymbolKind = "external"
)

// Symbol is one resolved declaration.
type Symbol struct {
	Name     string
	Kind     SymbolKind
	Dialect  Dialect
	Package  string
	Exported bool
	// Keyword is the root keyword as written, which distinguishes a component
	// from a statement without a second field to keep in step.
	Keyword string
	Params  []Param
	Output  string
	Fields  []Field
	// GoFunc is the generated Go function a declaration produces, when it
	// produces one. It is the exported name the call site in handwritten Go
	// uses, which is what makes hover useful before navigation exists.
	GoFunc string

	URI       string
	Container string
	Range     Range
}

// fileSymbols is one source's contribution to the graph.
type fileSymbols struct {
	pkg     string
	imports []string
	symbols []Symbol
}

// TypeGraph is every declaration the project's purposes list, grouped so a
// name can be resolved from the file that wrote it.
type TypeGraph struct {
	// byPackage holds the declarations of each package, in declaration order.
	byPackage map[string][]Symbol
	// byFile records what each file declares and what it imports, which is
	// what decides the names visible in it.
	byFile map[string]fileSymbols
}

func newTypeGraph() *TypeGraph {
	return &TypeGraph{byPackage: map[string][]Symbol{}, byFile: map[string]fileSymbols{}}
}

func (g *TypeGraph) add(uri string, file fileSymbols) {
	g.byFile[uri] = file
	g.byPackage[file.pkg] = append(g.byPackage[file.pkg], file.symbols...)
}

// Visible returns the declarations a file can name: its own package's, and the
// exported declarations of each package it imports.
//
// An import path is matched by its last segment, which is the package name a
// template writes. Resolving the path itself would need the module graph, and
// a template names a package rather than a path in every position that matters.
func (g *TypeGraph) Visible(uri string) []Symbol {
	file, known := g.byFile[uri]
	if !known {
		return nil
	}
	visible := append([]Symbol{}, g.byPackage[file.pkg]...)
	for _, path := range file.imports {
		name := path
		if index := strings.LastIndex(path, "/"); index >= 0 {
			name = path[index+1:]
		}
		if name == file.pkg {
			continue
		}
		for _, symbol := range g.byPackage[name] {
			if symbol.Exported {
				visible = append(visible, symbol)
			}
		}
	}
	return visible
}

// Resolve finds the declaration a name refers to from one file.
//
// A name declared in the file's own package wins over an imported one, which
// is the rule the generator applies: a local declaration shadows an import.
func (g *TypeGraph) Resolve(uri, name string) (Symbol, bool) {
	if name == "" {
		return Symbol{}, false
	}
	file, known := g.byFile[uri]
	if known {
		for _, symbol := range g.byPackage[file.pkg] {
			if symbol.Name == name {
				return symbol, true
			}
		}
	}
	for _, symbol := range g.Visible(uri) {
		if symbol.Name == name {
			return symbol, true
		}
	}
	return Symbol{}, false
}

// symbolsOf turns one parsed source into its contribution to the graph.
func symbolsOf(project *Project, path, uri string, kind Dialect, text string, found analysis, starts lineStarts) fileSymbols {
	file := fileSymbols{pkg: packageOf(found, path)}
	container := filepath.ToSlash(relativeTo(project.Root, path))

	if found.module != nil {
		for _, declared := range found.module.Imports {
			file.imports = append(file.imports, declared.Path)
		}
	}

	// The lowered Go types, when the module analyzes cleanly. A buffer being
	// edited usually does not, and a missing Go type costs a line of hover
	// rather than the answer.
	lowered := map[string]htmlbind.Signature{}
	if kind == dialectHTML {
		if signatures, err := htmlbind.Signatures(filepath.Base(path), []byte(text)); err == nil {
			for _, signature := range signatures {
				lowered[signature.Name] = signature
			}
		}
	}

	for _, declaration := range moduleDeclarations(found.module) {
		if symbol, ok := symbolOf(declaration, text, starts); ok {
			symbol.Dialect, symbol.Package, symbol.URI, symbol.Container = kind, file.pkg, uri, container
			applySignature(&symbol, lowered)
			file.symbols = append(file.symbols, symbol)
		}
	}
	for _, query := range found.dynamo {
		symbol := Symbol{
			Name:      query.Name,
			Kind:      kindStatement,
			Keyword:   "statement",
			Exported:  query.Exported,
			Output:    "dynamo." + string(query.Shape) + "<" + query.ItemType + ">",
			Dialect:   kind,
			Package:   file.pkg,
			URI:       uri,
			Container: container,
			Range:     declaredNameRange(query.Line, 1, query.Name, text, starts),
			GoFunc:    query.Name,
		}
		for _, parameter := range query.Params {
			symbol.Params = append(symbol.Params, Param{Name: parameter.Name, Type: parameter.Type})
		}
		file.symbols = append(file.symbols, symbol)
	}
	return file
}

// packageOf is the package a source declares, or the directory it sits in.
// A .pw.html carries no package line of its own in every project; the
// directory is what api:cli-generate compiles it into either way.
func packageOf(found analysis, path string) string {
	if found.module != nil && found.module.Package != nil && found.module.Package.Name != "" {
		return found.module.Package.Name
	}
	return filepath.Base(filepath.Dir(path))
}

func symbolOf(declaration sqlbind.Declaration, text string, starts lineStarts) (Symbol, bool) {
	switch node := declaration.(type) {
	case *sqlbind.TemplateDecl:
		keyword := rootKeyword(node.Kind)
		symbol := Symbol{
			Name:     node.Name,
			Kind:     kindStatement,
			Keyword:  keyword,
			Exported: node.Exported,
			Output:   typeName(node.Output),
			Range:    declaredNameRange(node.Pos.Line, node.Pos.Col, node.Name, text, starts),
		}
		if keyword == "component" {
			symbol.Kind = kindComponent
		}
		if node.Exported {
			symbol.GoFunc = node.Name
		}
		for _, parameter := range node.Parameters {
			symbol.Params = append(symbol.Params, Param{Name: parameter.Name, Type: typeName(parameter.Type)})
		}
		return symbol, true
	case *sqlbind.TypeDecl:
		// A record and an enum have no export modifier: naming one is what
		// makes it reachable, so the graph treats every one as visible.
		symbol := Symbol{Name: node.Name, Kind: kindType, Keyword: "type", Exported: true,
			Range: declaredNameRange(node.Pos.Line, node.Pos.Col, node.Name, text, starts)}
		for _, field := range node.Fields {
			symbol.Fields = append(symbol.Fields, Field{Name: field.Name, Type: typeName(field.Type), Line: field.Pos.Line})
		}
		return symbol, true
	case *sqlbind.EnumDecl:
		symbol := Symbol{Name: node.Name, Kind: kindEnum, Keyword: "enum", Exported: true,
			Range: declaredNameRange(node.Pos.Line, node.Pos.Col, node.Name, text, starts)}
		for _, member := range node.Members {
			symbol.Fields = append(symbol.Fields, Field{Name: member.Name, Line: member.Pos.Line})
		}
		return symbol, true
	case *sqlbind.ExternalDecl:
		symbol := Symbol{Name: node.Name, Kind: kindExternal, Keyword: "external", Exported: true,
			Output: typeName(node.Result), Range: declaredNameRange(node.Pos.Line, node.Pos.Col, node.Name, text, starts)}
		for _, parameter := range node.Parameters {
			symbol.Params = append(symbol.Params, Param{Name: parameter.Name, Type: typeName(parameter.Type)})
		}
		return symbol, true
	default:
		return Symbol{}, false
	}
}

// applySignature fills the lowered Go types onto a declaration the analysis
// resolved. A parameter the analysis did not name keeps the type as written.
func applySignature(symbol *Symbol, lowered map[string]htmlbind.Signature) {
	signature, resolved := lowered[symbol.Name]
	if !resolved {
		return
	}
	for index := range symbol.Params {
		for _, parameter := range signature.Parameters {
			if parameter.Name != symbol.Params[index].Name {
				continue
			}
			symbol.Params[index].GoType = parameter.GoType
			symbol.Params[index].Slot = parameter.Slot
		}
	}
}

// Signature renders a declaration the way its source spells it, which is what
// a hover shows before anything about the generated Go.
func (s Symbol) Signature() string {
	var builder strings.Builder
	// Only a component or a statement carries the modifier in the source. A
	// type is addressable from another package without one, so Exported is
	// true for the graph's benefit and printing it would quote a word the
	// declaration does not have.
	if s.Exported && (s.Kind == kindComponent || s.Kind == kindStatement) {
		builder.WriteString("export ")
	}
	builder.WriteString(s.Keyword)
	builder.WriteString(" ")
	builder.WriteString(s.Name)
	switch s.Kind {
	case kindType, kindEnum:
		return builder.String()
	}
	parts := make([]string, 0, len(s.Params))
	for _, parameter := range s.Params {
		parts = append(parts, parameter.Name+": "+parameter.Type)
	}
	builder.WriteString("(" + strings.Join(parts, ", ") + ")")
	if s.Output != "" {
		builder.WriteString(": " + s.Output)
	}
	return builder.String()
}

// rootKeyword is the declaration keyword as an author writes it. The parser
// namespaces the node type by dialect, so a .pw.sql statement arrives as
// "sql:statement"; quoting that back at a reader would show a name the
// language does not have.
func rootKeyword(kind string) string {
	if _, after, found := strings.Cut(kind, ":"); found {
		return after
	}
	return kind
}
