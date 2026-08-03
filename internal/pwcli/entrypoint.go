package pwcli

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The entry point is the one application-owned file a capability has to reach
// into: a store is opt-in by blank import, so configuration that names a backend
// does nothing until the binary links it. api:cli-init writes those lines, and
// api:cli-add printing them instead left a project that took a capability later
// in a different state from one that took it at bootstrap — which is the whole
// promise of requirement:incremental-project-capabilities.
//
// Every edit here is planned, shown on the review screen with the file it
// changes, and applied only after that screen is accepted. What keeps it safe is
// not that the change is small but that it is spliced in at a position the
// parser found: the rest of the file is copied through byte for byte, so
// comments, grouping, and hand-tuned formatting survive.

// entryPointSource finds the file in the main package that declares func main.
// A package with none is an error rather than a silent skip: the caller is about
// to plan an edit to it.
func entryPointSource(root, mainPackage string) (string, string, error) {
	directory := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(mainPackage, "./")))
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", "", err
	}
	fileset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_pw_gen.go") {
			continue
		}
		path := filepath.Join(directory, name)
		source, err := os.ReadFile(path)
		if err != nil {
			return "", "", err
		}
		file, err := parser.ParseFile(fileset, path, source, parser.SkipObjectResolution)
		if err != nil {
			return "", "", err
		}
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.Name == "main" {
				relative, err := filepath.Rel(root, path)
				if err != nil {
					return "", "", err
				}
				return filepath.ToSlash(relative), string(source), nil
			}
		}
	}
	return "", "", fmt.Errorf("no func main in %s", mainPackage)
}

// blankImport is one import to add, with the comment that says what it is for.
// The comment matters more than usual here: a blank import names a package
// nothing in the file calls, so without one the next reader has to go and find
// out why it is there.
type blankImport struct {
	path    string
	comment string
}

// withBlankImports splices imports into the file's import block, skipping any it
// already has. The result is gofmt'd, so the new lines join the grouping the
// file already uses rather than inventing one.
func withBlankImports(source string, imports ...blankImport) (string, error) {
	var pending []blankImport
	for _, wanted := range imports {
		if !importsPath(source, wanted.path) {
			pending = append(pending, wanted)
		}
	}
	if len(pending) == 0 {
		return source, nil
	}
	fileset := token.NewFileSet()
	file, err := parser.ParseFile(fileset, "main.go", source, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return "", err
	}
	var block *ast.GenDecl
	for _, decl := range file.Decls {
		if generic, ok := decl.(*ast.GenDecl); ok && generic.Tok == token.IMPORT && generic.Lparen.IsValid() {
			block = generic
			break
		}
	}
	if block == nil {
		// A single unparenthesized import, or none at all. Rewriting that shape
		// is a different edit, and guessing at it is how a scaffold corrupts a
		// file it was asked to help with.
		return "", fmt.Errorf("no parenthesized import block to add to")
	}
	var addition strings.Builder
	for _, wanted := range pending {
		addition.WriteString("\n")
		if wanted.comment != "" {
			addition.WriteString("\t// " + wanted.comment + "\n")
		}
		addition.WriteString("\t_ " + strconv.Quote(wanted.path) + "\n")
	}
	offset := fileset.Position(block.Rparen).Offset
	return formatted(source[:offset] + addition.String() + source[offset:])
}

// withMainCall makes call the first statement of func main, or returns the
// source unchanged when it is already there.
func withMainCall(source, call, comment string) (string, error) {
	if strings.Contains(source, call) {
		return source, nil
	}
	fileset := token.NewFileSet()
	file, err := parser.ParseFile(fileset, "main.go", source, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return "", err
	}
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok || function.Recv != nil || function.Name.Name != "main" || function.Body == nil {
			continue
		}
		addition := "\n"
		if comment != "" {
			addition += "\t// " + comment + "\n"
		}
		addition += "\t" + call
		offset := fileset.Position(function.Body.Lbrace).Offset + 1
		return formatted(source[:offset] + addition + source[offset:])
	}
	return "", fmt.Errorf("no func main to call %s from", call)
}

// importsPath reports whether the file already imports a path, blank or not, so
// running a capability twice does not stack duplicates.
func importsPath(source, path string) bool {
	return strings.Contains(source, strconv.Quote(path))
}

func formatted(source string) (string, error) {
	out, err := format.Source([]byte(source))
	if err != nil {
		return "", err
	}
	return string(out), nil
}
