package pwcli

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

// storybookFileName is the generated registration file, one per template
// package. It carries the pwdev build constraint, so api:cli-build emits and
// links none of it.
const storybookFileName = "popcornwave_storybook_pw_gen.go"

// storybookDirectory holds the generated harness main.
//
// The leading dot is load bearing. The go tool skips such directories when it
// expands a wildcard, so `go build ./...` and `go vet ./...` never see a main
// package whose only file is excluded by a build constraint — which would
// otherwise be an error in every project that has one. An explicit path still
// reaches it, which is how pw runs it.
const storybookDirectory = ".pwstorybook"

// storybookTemplate is one generated template function found in a package.
type storybookTemplate struct {
	Package string
	Name    string
	Params  string
	// Document marks the shell binder, which the harness registers separately
	// so a story can be rendered inside it.
	Document bool
}

func (t storybookTemplate) exported() bool {
	return t.Name != "" && t.Name[0] >= 'A' && t.Name[0] <= 'Z'
}

// plannedSources indexes what this generation run is about to write, so the
// storybook scan reads the output being produced rather than the output of the
// previous run. Check mode depends on this: it has to report drift without
// having written anything.
type plannedSources map[string][]byte

func planned(changes []fileChange) plannedSources {
	sources := plannedSources{}
	for _, change := range changes {
		if !change.remove {
			sources[change.path] = change.source
		}
	}
	return sources
}

// read returns the planned content of a file, or its content on disk when this
// run leaves it alone.
func (p plannedSources) read(path string) ([]byte, error) {
	if source, ok := p[path]; ok {
		return source, nil
	}
	return os.ReadFile(path)
}

// scanStorybookTemplates reads the generated files of one package and reports
// the templates it binds.
//
// It reads the generated Go rather than the .pw.html source because what the
// harness has to call is the generated function and its parameter type, and
// those are facts about the output. A template whose generation failed is
// therefore absent rather than registered and broken.
func scanStorybookTemplates(directory string, sources plannedSources) ([]storybookTemplate, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			names[entry.Name()] = true
		}
	}
	// A file this run is creating is not on disk yet, so the planned set is
	// consulted for names as well as for content.
	for path := range sources {
		if filepath.Dir(path) == directory {
			names[filepath.Base(path)] = true
		}
	}
	var templates []storybookTemplate
	packageName := ""
	for name := range names {
		if !strings.HasSuffix(name, "_pw_gen.go") || name == storybookFileName {
			continue
		}
		source, err := sources.read(filepath.Join(directory, name))
		if err != nil {
			continue
		}
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, name, source, 0)
		if err != nil {
			// A generated file that does not parse is a generation defect the
			// build will report far more clearly than this scan would.
			continue
		}
		packageName = file.Name.Name
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil {
				continue
			}
			if name, params, ok := templateBinder(function); ok {
				templates = append(templates, storybookTemplate{
					Package: packageName, Name: name, Params: params,
				})
				continue
			}
			if name, params, ok := documentBinder(function); ok {
				templates = append(templates, storybookTemplate{
					Package: packageName, Name: name, Params: params, Document: true,
				})
			}
		}
	}
	sort.Slice(templates, func(i, j int) bool { return templates[i].Name < templates[j].Name })
	return templates, nil
}

// templateBinder matches `func X(params XParams) htmlbind.Fragment`, which is
// the shape every generated template has.
func templateBinder(function *ast.FuncDecl) (name, params string, ok bool) {
	if !singleResult(function, "Fragment") || function.Type.Params == nil || len(function.Type.Params.List) != 1 {
		return "", "", false
	}
	identifier, ok := function.Type.Params.List[0].Type.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	return function.Name.Name, identifier.Name, true
}

// documentBinder matches `func BindX(params XParams) htmlbind.Wrapper`, the
// chain binder generated for a component with an unnamed slot.
func documentBinder(function *ast.FuncDecl) (name, params string, ok bool) {
	if !singleResult(function, "Wrapper") || function.Type.Params == nil || len(function.Type.Params.List) != 1 {
		return "", "", false
	}
	identifier, ok := function.Type.Params.List[0].Type.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	return function.Name.Name, identifier.Name, true
}

func singleResult(function *ast.FuncDecl, typeName string) bool {
	if function.Type.Results == nil || len(function.Type.Results.List) != 1 {
		return false
	}
	selector, ok := function.Type.Results.List[0].Type.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	packageIdent, ok := selector.X.(*ast.Ident)
	return ok && packageIdent.Name == "htmlbind" && selector.Sel.Name == typeName
}

// storybookRegistration is the file generated into a template package.
//
// It lives in the package rather than importing it because a generated fragment
// named for an unexported template is unreachable from anywhere else. Putting
// the registration inside means the storybook lists what the project generated
// rather than the subset it chose to export.
func storybookRegistration(templates []storybookTemplate) string {
	var out strings.Builder
	packageName := templates[0].Package
	out.WriteString("//go:build pwdev\n\n")
	out.WriteString("// Code generated by pw; DO NOT EDIT.\n\n")
	out.WriteString("package " + packageName + "\n\n")
	out.WriteString("import (\n")
	out.WriteString("\t\"github.com/shibukawa/popcornwave/pwstory\"\n")
	out.WriteString("\t\"github.com/shibukawa/tinybind-go/htmlbind\"\n")
	out.WriteString(")\n\n")
	out.WriteString("func init() {\n")
	for _, template := range templates {
		if template.Document {
			fmt.Fprintf(&out, "\tpwstory.RegisterDocument(%s(%s{}))\n", template.Name, template.Params)
			continue
		}
		fmt.Fprintf(&out, "\tpwstory.Register(pwstory.Template{\n")
		fmt.Fprintf(&out, "\t\tPackage:   %q,\n", template.Package)
		fmt.Fprintf(&out, "\t\tName:      %q,\n", template.Name)
		fmt.Fprintf(&out, "\t\tExported:  %t,\n", template.exported())
		fmt.Fprintf(&out, "\t\tNewParams: func() any { return new(%s) },\n", template.Params)
		fmt.Fprintf(&out, "\t\tRender:    func(params any) htmlbind.Fragment { return %s(*params.(*%s)) },\n",
			template.Name, template.Params)
		fmt.Fprintf(&out, "\t})\n")
	}
	out.WriteString("}\n")
	return out.String()
}

// storybookHarness is the generated main: a list of imports and one call, so
// everything it does lives in pwstory where it can be tested without generating
// a project first.
func storybookHarness(module string, packages []string) string {
	var out strings.Builder
	out.WriteString("//go:build pwdev\n\n")
	out.WriteString("// Code generated by pw; DO NOT EDIT.\n\n")
	out.WriteString("package main\n\n")
	out.WriteString("import (\n")
	out.WriteString("\t\"log\"\n\n")
	out.WriteString("\t\"github.com/shibukawa/popcornwave/pwstory\"\n")
	for _, path := range packages {
		fmt.Fprintf(&out, "\t_ %q\n", module+"/"+path)
	}
	out.WriteString(")\n\n")
	out.WriteString("func main() {\n")
	out.WriteString("\tif err := pwstory.ListenAndServe(); err != nil {\n")
	out.WriteString("\t\tlog.Fatal(err)\n")
	out.WriteString("\t}\n")
	out.WriteString("}\n")
	return out.String()
}

// planStorybook adds the registration file of every template package and the
// harness main that imports them.
//
// It runs after the templates themselves are planned, because it reads the
// generated files to find what to register. A project with no template tree
// plans nothing here and gets no harness.
func planStorybook(root string, config projectConfig, changes []fileChange) ([]fileChange, error) {
	moduleSource, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	modulePath := modfile.ModulePath(moduleSource)
	if modulePath == "" {
		return nil, fmt.Errorf("go.mod does not declare a module path")
	}
	sources := planned(changes)
	// Every directory holding a generated template, from both purposes: a page
	// tree generates templates the same way a templates directory does.
	directories := map[string]bool{}
	for _, purpose := range [][]string{config.Generate.Templates, config.Generate.Pages} {
		err := walkSources(root, purpose, func(path string, entry fs.DirEntry) error {
			if strings.HasSuffix(entry.Name(), ".pw.html") {
				directories[filepath.Dir(path)] = true
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	var packages []string
	for directory := range directories {
		templates, err := scanStorybookTemplates(directory, sources)
		if err != nil || len(templates) == 0 {
			continue
		}
		changes, err = appendIfChanged(changes,
			filepath.Join(directory, storybookFileName),
			[]byte(storybookRegistration(templates)))
		if err != nil {
			return nil, err
		}
		relative, err := filepath.Rel(root, directory)
		if err != nil {
			return nil, err
		}
		packages = append(packages, filepath.ToSlash(relative))
	}
	if len(packages) == 0 {
		return changes, nil
	}
	sort.Strings(packages)
	return appendIfChanged(changes,
		filepath.Join(root, storybookDirectory, "main_pw_gen.go"),
		[]byte(storybookHarness(modulePath, packages)))
}
