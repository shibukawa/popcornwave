package pwcli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shibukawa/popcornwave/internal/pwgen"
	"github.com/shibukawa/tinybind-go/routetree"
)

// The rungs a page can be written at. One file is a page; a signature beside it
// is what raises it.
const (
	// pageRungTemplate is the page template alone. The handler is generated
	// whole, and the data comes from the template's own external calls.
	pageRungTemplate = "template"
	// pageRungLoader adds an external the template declares and binds, whose
	// parameters are the route's inputs and whose result the component renders.
	// It is a shape rather than a level: the handler is still generated whole,
	// and what the page gains is Go of its own to load with.
	pageRungLoader = "loader"
	// pageRungHandler adds a Load that takes the writer and the request, so the
	// response is the application's from that point on.
	pageRungHandler = "handler"
)

// pageDestinations lists the page tree roots a route may be added to.
func pageDestinations(state projectState) []string {
	return append([]string(nil), state.config.Generate.Pages...)
}

// preselectedPageTree answers with the tree the working directory is in,
// because that is where the operator already is.
func preselectedPageTree(state projectState, destinations []string) string {
	if len(destinations) == 0 {
		return ""
	}
	working, err := os.Getwd()
	if err != nil {
		return destinations[0]
	}
	relative, err := filepath.Rel(state.root, working)
	if err != nil {
		return destinations[0]
	}
	current := filepath.ToSlash(relative)
	for _, root := range destinations {
		if current == root || strings.HasPrefix(current, root+"/") {
			return root
		}
	}
	return destinations[0]
}

// pageSegment is one URL segment of a new route, in both spellings.
type pageSegment struct {
	// Directory is what the filesystem holds: users, id_, or rest__.
	Directory string
	// Param names the dynamic segment, or is empty for a static one.
	Param string
	// CatchAll marks the two-underscore form, which binds the rest of the path.
	CatchAll bool
}

// parsePagePath converts a route path into the directories that serve it.
//
// The dynamic spellings are the ones a router usually writes, because that is
// what an operator will type; the underscore form they become is a property of
// the filesystem, per rule:page-directory-naming.
func parsePagePath(path string) ([]pageSegment, error) {
	trimmed := strings.Trim(strings.TrimSpace(path), "/")
	if trimmed == "" {
		return nil, nil
	}
	var segments []pageSegment
	for _, part := range strings.Split(trimmed, "/") {
		switch {
		case part == "":
			return nil, fmt.Errorf("%q has an empty segment", path)
		case strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}"):
			name := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			catchAll := strings.HasSuffix(name, "...")
			name = strings.TrimSuffix(name, "...")
			if name == "" {
				return nil, fmt.Errorf("%q has an unnamed dynamic segment", path)
			}
			directory := name + "_"
			if catchAll {
				directory = name + "__"
			}
			if err := routetree.ValidateDirName(directory); err != nil {
				return nil, fmt.Errorf("segment %q: %w", part, err)
			}
			segments = append(segments, pageSegment{Directory: directory, Param: name, CatchAll: catchAll})
		default:
			if err := routetree.ValidateDirName(part); err != nil {
				return nil, fmt.Errorf("segment %q: %w", part, err)
			}
			segments = append(segments, pageSegment{Directory: part})
		}
	}
	for index, segment := range segments {
		if segment.CatchAll && index != len(segments)-1 {
			return nil, fmt.Errorf("%q puts a catch-all before the end of the path", path)
		}
	}
	return segments, nil
}

// pageRouteDirectory is the tree-relative directory a route path serves from.
func pageRouteDirectory(root string, segments []pageSegment) string {
	parts := make([]string, 0, len(segments)+1)
	parts = append(parts, root)
	for _, segment := range segments {
		parts = append(parts, segment.Directory)
	}
	return strings.Join(parts, "/")
}

// planPage computes the sources one route needs: its page template, the Go
// entry point of the rung it was asked for, and a layout when the tree has none
// above it yet.
//
// Nothing registers a page, so unlike a handler there is no mux to create and
// nothing to print about wiring: the directory is the registration.
func planPage(state projectState, options newOptions) (*capabilityPlan, error) {
	plan := newCapabilityPlan()
	segments, err := parsePagePath(options.Path)
	if err != nil {
		return nil, err
	}
	directory := pageRouteDirectory(options.Package, segments)
	if _, err := os.Stat(filepath.Join(state.root, filepath.FromSlash(directory), pwgen.PageFile)); err == nil {
		return nil, fmt.Errorf("%s already serves that route", directory)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	pkg := routetree.PackageName(filepath.Base(directory))
	plan.creates[directory+"/"+pwgen.PageFile] = pageTemplateSource(pkg, options.Rung, segments)
	if source := pageLogicSource(pkg, options.Rung, segments); source != "" {
		plan.creates[directory+"/page.go"] = source
	}
	if options.Layout {
		layout := options.Package + "/" + pwgen.LayoutFile
		if _, err := os.Stat(filepath.Join(state.root, filepath.FromSlash(layout))); os.IsNotExist(err) {
			plan.creates[layout] = pageLayoutSource(routetree.PackageName(filepath.Base(options.Package)))
		} else if err != nil {
			return nil, err
		}
	}
	plan.generate = true
	return plan, nil
}

// pageInputs names the component parameters a route's own segments supply. The
// leading parameters are the dynamic segments in route order, which is the rule
// whether the list is read from the component or from Load.
func pageInputs(segments []pageSegment) []string {
	var inputs []string
	for _, segment := range segments {
		if segment.Param != "" {
			inputs = append(inputs, segment.Param)
		}
	}
	return inputs
}

// indentTemplateBody puts the body of a brace-delimited template block one
// level in. The component brace is a block like any other, and leaving its body
// at column zero made the one block whose nesting was invisible the outermost
// one, so the markup read as if it sat beside the component rather than inside.
func indentTemplateBody(body string) string {
	lines := strings.Split(body, "\n")
	for index, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines[index] = "  " + line
	}
	return strings.Join(lines, "\n")
}

func pageTemplateSource(pkg, rung string, segments []pageSegment) string {
	var params, body string
	switch rung {
	case pageRungLoader:
		inputs := pageInputs(segments)
		declared := make([]string, 0, len(inputs))
		for _, input := range inputs {
			declared = append(declared, input+": string")
		}
		params = strings.Join(declared, ", ")
		// The loader is declared here and bound here, so the page's own source
		// says where its data comes from. The binding is evaluated before the
		// first byte, which is what lets a failing loader choose the status.
		binding := "{val greeting = LoadGreeting(" + strings.Join(inputs, ", ") + ")}"
		return "package " + pkg + `

external LoadGreeting(` + params + `): string

export component Page(` + params + `): html {
` + indentTemplateBody(binding+"\n<h1>{greeting}</h1>") + `
}
`
	default:
		declared := make([]string, 0, len(segments))
		shown := make([]string, 0, len(segments))
		for _, input := range pageInputs(segments) {
			declared = append(declared, input+": string")
			shown = append(shown, "<p>"+input+": {"+input+"}</p>")
		}
		params = strings.Join(declared, ", ")
		body = "<h1>new page</h1>"
		if len(shown) > 0 {
			body += "\n" + strings.Join(shown, "\n")
		}
	}
	return "package " + pkg + `

export component Page(` + params + `): html {
` + indentTemplateBody(body) + `
}
`
}

func pageLogicSource(pkg, rung string, segments []pageSegment) string {
	switch rung {
	case pageRungLoader:
		inputs := pageInputs(segments)
		parameters := make([]string, 0, len(inputs))
		for _, input := range inputs {
			parameters = append(parameters, input+" string")
		}
		value := `"hello"`
		if len(inputs) > 0 {
			value = `"hello, " + ` + inputs[0]
		}
		return "package " + pkg + `

// LoadGreeting is the page's own loader. The template declares it as an
// external and binds it with {val}, so the call site is in the page rather than
// in generated code.
//
// The trailing error is what lets it choose the response: the binding runs
// before the first byte, so returning an error carrying HTTP intent still
// selects the status while the rest of the page streams.
//
// Declare a leading context.Context to receive the request's, which is where
// the database handle and the signed-in reader live.
func LoadGreeting(` + strings.Join(parameters, ", ") + `) (string, error) {
	return ` + value + `, nil
}
`
	case pageRungHandler:
		return "package " + pkg + `

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

// Load owns the whole response. Only the registration is generated for this
// rung, so composing the page inside its layouts is this function's job. Render
// is the composer generated beside this file; it takes the route's own inputs
// and the page's parameters, and puts the layout chain and the document shell
// around the result.
func Load(w http.ResponseWriter, r *http.Request) {
	route, err := DecodeRoute(r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if err := Render(w, r, route, PageParams{}); err != nil {
		pw.WriteProblem(w, r, err)
	}
}
`
	default:
		return ""
	}
}

func pageLayoutSource(pkg string) string {
	return "package " + pkg + `

// A layout wraps every page below it. Declaring children as html is what makes
// the template compiler emit the wrapper the generated chain calls.
export component Layout(children: html): html {
  <div class="page"><slot required /></div>
}
`
}
