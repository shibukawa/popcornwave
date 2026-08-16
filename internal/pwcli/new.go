package pwcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shibukawa/popcornwave/internal/pwgen"
)

// The implemented kinds. A third one costs an entry here, a step list, and its
// scaffold branch.
//
// A kind is named after the source it writes, never after the router that will
// serve it: pw add installs a registered or a discovered router, and pw new adds
// a handler or a page to one that is already there. The two commands work at
// different levels, so they use different words on purpose.
const (
	newKindHandler = "handler"
	// newKindPage adds a route to a concept:page-tree. It is offered only to a
	// project that has one, because a route with no tree to live in is nothing.
	newKindPage = "page"
)

var newKinds = []string{newKindHandler, newKindPage}

const newUsage = "usage: pw new [" + newKindHandler + "|" + newKindPage + "]"

// newOptions holds every answer the wizard collects.
type newOptions struct {
	Kind string
	// Package is the destination directory, project-relative in slash form.
	Package string
	Method  string
	Path    string
	Name    string
	// HTML renders a typed page; otherwise the handler answers with JSON.
	HTML bool
	// Input scaffolds a request type and the pw.Parse call that fills it.
	Input bool
	// Rung selects how much Go runs between the request and the render of a
	// page. It applies to the page kind only.
	Rung string
	// Layout writes the tree's layout when it has none yet.
	Layout bool
}

func runNew(ctx context.Context, args []string, stdout io.Writer) error {
	root, err := projectRoot(".")
	if err != nil {
		return err
	}
	state, err := loadProjectState(root)
	if err != nil {
		return err
	}
	options := newOptions{Kind: newKindHandler, Method: "GET", HTML: true}
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("new: %s takes no options; %s", arg, newUsage)
		}
		if !slices.Contains(newKinds, arg) {
			return fmt.Errorf("new: unknown kind %q; %s", arg, newUsage)
		}
		options.Kind = arg
	}
	var destinations []string
	if options.Kind == newKindPage {
		destinations = pageDestinations(state)
		if len(destinations) == 0 {
			// The kinds this command writes are named after the source — a page,
			// a handler — while the routers that serve them are named registered
			// and discovered. Installing one is pw add's job, so the message
			// crosses from one vocabulary to the other rather than mixing them.
			return fmt.Errorf("new: this project serves no %s; run pw add %s, or list a tree root in generate.pages",
				newKindPage+" tree", capabilityDiscovered)
		}
		options.Package = preselectedPageTree(state, destinations)
		options.Rung = pageRungTemplate
	} else {
		destinations, err = handlerDestinations(state)
		if err != nil {
			return err
		}
		if len(destinations) == 0 {
			return fmt.Errorf("new: this project serves no %s package; run pw add %s, or list a directory in generate.handlers",
				newKindHandler, capabilityRegistered)
		}
		options.Package = preselectedPackage(state, destinations)
	}
	if !interactiveTerminal() {
		return fmt.Errorf("new: the wizard needs a terminal; %s", newUsage)
	}
	options, err = runNewWizard(state, destinations, options)
	if errors.Is(err, errWizardCanceled) {
		fmt.Fprintln(stdout, "new canceled")
		return nil
	}
	if err != nil {
		return err
	}
	plan, err := planNewSource(state, options)
	if err != nil {
		return err
	}
	if err := plan.apply(state.root); err != nil {
		return err
	}
	for _, line := range plan.summary() {
		fmt.Fprintln(stdout, " ", line)
	}
	if _, err := generateProject(ctx, false, stdout, false); err != nil {
		// The written sources are handwritten code the operator owns and fixes,
		// so they stay; only the generated artifacts are missing.
		return fmt.Errorf("generate %s: %w", options.Package, err)
	}
	if options.Kind == newKindPage {
		fmt.Fprintf(stdout, "\nAdded GET %s\n", pageRoutePattern(options.Path))
		return nil
	}
	fmt.Fprintf(stdout, "\nAdded %s %s\n", options.Method, options.Path)
	return nil
}

// planNewSource routes one answer set to the scaffold of its kind.
func planNewSource(state projectState, options newOptions) (*capabilityPlan, error) {
	if options.Kind == newKindPage {
		return planPage(state, options)
	}
	return planHandler(state, options)
}

// pageRoutePattern normalizes what the operator typed into the path the route
// is actually served under.
func pageRoutePattern(path string) string {
	trimmed := "/" + strings.Trim(strings.TrimSpace(path), "/")
	if trimmed == "/" {
		return "/{$}"
	}
	return trimmed
}

// handlerDestinations lists the directories a handler may go in: the
// generate.handlers entries and the packages already inside them.
func handlerDestinations(state projectState) ([]string, error) {
	var found []string
	for _, source := range state.config.Generate.Handlers {
		found = append(found, source)
		entries, err := os.ReadDir(filepath.Join(state.root, filepath.FromSlash(source)))
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
				found = append(found, source+"/"+entry.Name())
			}
		}
	}
	slices.Sort(found)
	return slices.Compact(found), nil
}

// preselectedPackage answers with the working directory when it can hold a
// handler, because that is where the operator already is.
func preselectedPackage(state projectState, destinations []string) string {
	working, err := os.Getwd()
	if err != nil {
		return destinations[0]
	}
	relative, err := filepath.Rel(state.root, working)
	if err != nil {
		return destinations[0]
	}
	current := filepath.ToSlash(relative)
	if slices.Contains(destinations, current) {
		return current
	}
	// A directory below a listed one is still inside the handler purpose.
	for _, source := range state.config.Generate.Handlers {
		if current == source || strings.HasPrefix(current, source+"/") {
			return current
		}
	}
	return destinations[0]
}

// planHandler computes the sources a handler needs: its own file, the mux when
// the package is new, and the page template when it renders HTML.
func planHandler(state projectState, options newOptions) (*capabilityPlan, error) {
	plan := newCapabilityPlan()
	directory := options.Package
	pattern := options.Method + " " + options.Path
	if owner, ok, err := routeAlreadyRegistered(state, directory, pattern); err != nil {
		return nil, err
	} else if ok {
		return nil, fmt.Errorf("%s is already registered in %s", pattern, owner)
	}
	pkg := goPackageIdentifier(directory)
	if _, err := os.Stat(filepath.Join(state.root, filepath.FromSlash(directory))); os.IsNotExist(err) {
		plan.creates[directory+"/index.go"] = muxScaffold(initOptions{TinyGo: state.config.Toolchain == toolchainTinyGo})
		plan.manual = append(plan.manual,
			"import "+directory+" from "+state.config.Main+" and mount its Handlers()")
	} else if err != nil {
		return nil, err
	} else if !hasHandlerMux(state.root, directory) {
		plan.creates[directory+"/index.go"] = muxScaffold(initOptions{TinyGo: state.config.Toolchain == toolchainTinyGo})
		plan.manual = append(plan.manual,
			"import "+directory+" from "+state.config.Main+" and mount its Handlers()")
	}
	plan.creates[directory+"/"+options.Name+"_handler.go"] = handlerSourceScaffold(pkg, options)
	if options.HTML {
		if !withinPurpose(state.config.Generate.Templates, directory) {
			return nil, fmt.Errorf("%s is outside generate.templates, so its page template would never be generated", directory)
		}
		plan.creates[directory+"/"+options.Name+".pw.html"] = handlerTemplateScaffold(pkg, options)
	}
	plan.generate = true
	return plan, nil
}

// withinPurpose reports whether a directory is covered by a purpose list.
func withinPurpose(sources []string, directory string) bool {
	for _, source := range sources {
		if directory == source || strings.HasPrefix(directory, source+"/") {
			return true
		}
	}
	return false
}

// hasHandlerMux reports whether a package already owns the mux new handlers
// register against.
func hasHandlerMux(root, directory string) bool {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(directory)))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(directory), entry.Name()))
		if err != nil {
			continue
		}
		if strings.Contains(string(source), "func Handlers()") {
			return true
		}
	}
	return false
}

// routeAlreadyRegistered finds the file that registers a pattern. Registration
// is a literal under rule:static-route-discovery, which is what makes a
// duplicate detectable without building the package.
func routeAlreadyRegistered(state projectState, directory, pattern string) (string, bool, error) {
	entries, err := os.ReadDir(filepath.Join(state.root, filepath.FromSlash(directory)))
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		name := filepath.Join(state.root, filepath.FromSlash(directory), entry.Name())
		source, err := os.ReadFile(name)
		if err != nil {
			return "", false, err
		}
		if strings.Contains(string(source), `"`+pattern+`"`) {
			return directory + "/" + entry.Name(), true, nil
		}
	}
	return "", false, nil
}

// handlerSourceScaffold writes the route registration and the handler body.
func handlerSourceScaffold(pkg string, options newOptions) string {
	exported := exportedName(options.Name)
	imports := "\t\"net/http\"\n\n\t\"github.com/shibukawa/popcornwave/pw\"\n"
	body := ""
	parse := ""
	input := ""
	if options.Input {
		input = "// " + options.Name + "Input is what this route reads from the request.\n" +
			"type " + options.Name + "Input struct {\n" +
			"\t// Name is the value the response echoes back. A field comment becomes\n" +
			"\t// the parameter description in the OpenAPI document.\n" +
			"\tName string `query:\"name\" default:\"World\"`\n" +
			"}\n\n"
		parse = "\tinput, err := pw.Parse[" + options.Name + "Input](r)\n" +
			"\tif err != nil {\n" +
			"\t\tpw.WriteProblem(w, r, pw.BadRequest(err))\n" +
			"\t\treturn\n" +
			"\t}\n"
	}
	switch {
	case options.HTML && options.Input:
		body = "\tpw.WriteHTML(w, r, " + exported + "(" + exported + "Params{Name: input.Name}))\n"
	case options.HTML:
		body = "\tpw.WriteHTML(w, r, " + exported + "(" + exported + "Params{Name: \"World\"}))\n"
	case options.Input:
		body = "\tpw.WriteAPI(w, r, http.StatusOK, " + exported + "Response{Name: input.Name})\n"
	default:
		body = "\tpw.WriteAPI(w, r, http.StatusOK, " + exported + "Response{Name: \"World\"})\n"
	}
	response := ""
	if !options.HTML {
		response = "// " + exported + "Response is what this route answers with.\n" +
			"type " + exported + "Response struct {\n" +
			"\t// Name is the greeting subject.\n" +
			"\tName string `json:\"name\"`\n}\n\n"
	}
	return "package " + pkg + "\n\nimport (\n" + imports + ")\n\n" +
		input + response +
		"func init() { mux.HandleFunc(\"" + options.Method + " " + options.Path + "\", " + options.Name + ") }\n\n" +
		"// pw generate reads the godoc below into the OpenAPI document: the first\n" +
		"// sentence becomes the operation summary and the rest its description, so\n" +
		"// what is written there is what /docs shows about this route. This\n" +
		"// paragraph is separated by a blank line and reaches no document.\n\n" +
		"// " + options.Name + " serves " + options.Method + " " + options.Path + ".\n" +
		"//\n" +
		"// Replace this sentence with what the route is for.\n" +
		"func " + options.Name + "(w http.ResponseWriter, r *http.Request) {\n" + parse + body + "}\n"
}

// handlerTemplateScaffold writes the page. It carries leaf content only; the
// document shell of requirement:nested-html-templates wraps it.
func handlerTemplateScaffold(pkg string, options newOptions) string {
	exported := exportedName(options.Name)
	return "package " + pkg + "\n\nexport component " + exported + "(name: string): html {\n" +
		"  <h1>Hello, {name}</h1>\n}\n"
}

// exportedName turns a handler name into the exported identifier generation
// gives its template component.
func exportedName(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// defaultHandlerName derives an identifier from the route, which is what an
// operator would have typed anyway.
func defaultHandlerName(method, path string) string {
	segments := strings.FieldsFunc(path, func(r rune) bool { return r == '/' })
	var name strings.Builder
	name.WriteString(strings.ToLower(method))
	for _, segment := range segments {
		segment = strings.Trim(segment, "{}.")
		if segment == "" {
			continue
		}
		name.WriteString(exportedName(identifierSegment(segment)))
	}
	if name.Len() == len(method) {
		name.WriteString("Index")
	}
	return name.String()
}

// identifierSegment keeps the letters and digits of a path segment.
func identifierSegment(segment string) string {
	var out strings.Builder
	for _, r := range segment {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func runNewWizard(state projectState, destinations []string, defaults newOptions, programOptions ...tea.ProgramOption) (newOptions, error) {
	return runWizard(newNewWizard(state, destinations, defaults), programOptions...)
}

func newNewWizard(state projectState, destinations []string, defaults newOptions) wizardModel[newOptions] {
	steps := newWizardSteps(state, destinations, defaults)
	title := "Popcorn Wave  new handler"
	if defaults.Kind == newKindPage {
		steps = newPageWizardSteps(state, destinations, defaults)
		title = "Popcorn Wave  new page"
	}
	return wizardModel[newOptions]{
		steps:    steps,
		defaults: defaults,
		title:    title,
		confirm:  "write",
		plan: func(options newOptions) []string {
			plan, err := planNewSource(state, options)
			if err != nil {
				return []string{"cannot write it: " + err.Error()}
			}
			return plan.summary()
		},
		theme: newWizardTheme(),
	}
}

// newPageWizardSteps asks what a route needs: where it lives, what it serves,
// and how much Go runs before it renders.
func newPageWizardSteps(state projectState, destinations []string, defaults newOptions) []wizardStep[newOptions] {
	trees := make([]wizardChoice[newOptions], 0, len(destinations))
	for _, destination := range destinations {
		trees = append(trees, wizardChoice[newOptions]{
			name:        destination,
			description: "a generate.pages tree root",
			apply:       func(target *newOptions) { target.Package = destination },
		})
	}
	steps := []wizardStep[newOptions]{
		newTextStep(
			"Path",
			"The URL this page answers: /about, /users/{id}, or /files/{rest...}. "+
				"A dynamic segment becomes a directory ending in an underscore, because it is also a Go package.",
			defaults.Path,
			"/about",
			validatePagePath,
			func(target *newOptions, value string) { target.Path = value },
		),
		newChoiceStep(
			"Go beside the page",
			"One file is a page. What you add beside it decides how much Go runs between the request and the render.",
			0,
			wizardChoice[newOptions]{
				name:        "None",
				description: "the template alone; its own external calls fetch the data",
				apply:       func(target *newOptions) { target.Rung = pageRungTemplate },
			},
			wizardChoice[newOptions]{
				name:        "Loader",
				description: "page.go loads what the template binds; the handler is still generated",
				apply:       func(target *newOptions) { target.Rung = pageRungLoader },
			},
			wizardChoice[newOptions]{
				name:        "Handler Load",
				description: "page.go owns the whole response; only the registration is generated",
				apply:       func(target *newOptions) { target.Rung = pageRungHandler },
			},
		),
	}
	if len(trees) > 1 {
		steps = append([]wizardStep[newOptions]{newChoiceStep(
			"Page tree",
			"Only generate.pages roots are offered; a page elsewhere is served by nothing.",
			max(slices.Index(destinations, defaults.Package), 0),
			trees...,
		)}, steps...)
	}
	// A tree with no layout above the new route renders the page on its own,
	// which is a working page but rarely the one wanted.
	if !hasPageLayout(state, defaults.Package) {
		steps = append(steps, newChoiceStep(
			"Layout",
			"This tree has no layout, so a page renders on its own. A layout wraps every page below it.",
			0,
			wizardChoice[newOptions]{
				name:        "Add one",
				description: "writes " + defaults.Package + "/" + pwgen.LayoutFile,
				apply:       func(target *newOptions) { target.Layout = true },
			},
			wizardChoice[newOptions]{
				name:        "No",
				description: "the page renders with no wrapper of its own",
				apply:       func(target *newOptions) { target.Layout = false },
			},
		))
	}
	return steps
}

// hasPageLayout reports whether a tree root already carries a layout.
func hasPageLayout(state projectState, root string) bool {
	if root == "" {
		return true
	}
	_, err := os.Stat(filepath.Join(state.root, filepath.FromSlash(root), pwgen.LayoutFile))
	return err == nil
}

// validatePagePath reports why a route path cannot become directories.
func validatePagePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("enter a path, such as /about")
	}
	_, err := parsePagePath(path)
	return err
}

func newWizardSteps(state projectState, destinations []string, defaults newOptions) []wizardStep[newOptions] {
	packages := make([]wizardChoice[newOptions], 0, len(destinations))
	for _, destination := range destinations {
		description := "inside generate.handlers"
		if hasHandlerMux(state.root, destination) {
			description = "an existing handler package"
		}
		packages = append(packages, wizardChoice[newOptions]{
			name:        destination,
			description: description,
			apply:       func(target *newOptions) { target.Package = destination },
		})
	}
	methods := make([]wizardChoice[newOptions], 0, 5)
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		methods = append(methods, wizardChoice[newOptions]{
			name:        method,
			description: "registers " + method + " on the package mux",
			apply:       func(target *newOptions) { target.Method = method },
		})
	}
	return []wizardStep[newOptions]{
		newChoiceStep(
			"Package",
			"Only the generate.handlers directories are offered; a route elsewhere is analyzed by nothing.",
			max(slices.Index(destinations, defaults.Package), 0),
			packages...,
		),
		newChoiceStep(
			"Method",
			"The method and path form one literal pattern, which is what route discovery reads.",
			max(slices.Index([]string{"GET", "POST", "PUT", "PATCH", "DELETE"}, defaults.Method), 0),
			methods...,
		),
		newTextStep(
			"Path",
			"A Go 1.22 pattern: /tasks, /tasks/{id}, or /assets/ for a subtree.",
			defaults.Path,
			"/tasks",
			validateRoutePath,
			func(target *newOptions, value string) {
				target.Path = value
				target.Name = defaultHandlerName(target.Method, value)
			},
		),
		newTextStep(
			"Name",
			"The handler function and the source file stem; the template component takes its exported form.",
			"",
			"derived from the route",
			validateHandlerName,
			func(target *newOptions, value string) {
				if value != "" {
					target.Name = value
				}
			},
		),
		newChoiceStep(
			"Response",
			"An HTML page is rendered through the document shell; an API answer is JSON.",
			yesNoCursor(defaults.HTML),
			wizardChoice[newOptions]{
				name:        "HTML page",
				description: "a .pw.html template beside the handler",
				apply:       func(target *newOptions) { target.HTML = true },
			},
			wizardChoice[newOptions]{
				name:        "JSON API",
				description: "a typed response struct written with pw.WriteAPI",
				apply:       func(target *newOptions) { target.HTML = false },
			},
		),
		newChoiceStep(
			"Request input",
			"A typed input is bound by pw.Parse, which is also what generation documents in OpenAPI.",
			yesNoCursor(defaults.Input),
			wizardChoice[newOptions]{
				name:        "Yes",
				description: "a private input type and the pw.Parse call that fills it",
				apply:       func(target *newOptions) { target.Input = true },
			},
			wizardChoice[newOptions]{
				name:        "No",
				description: "a handler that reads nothing from the request",
				apply:       func(target *newOptions) { target.Input = false },
			},
		),
	}
}

// validateRoutePath rejects what route discovery could not read as a literal.
func validateRoutePath(value string) error {
	if !strings.HasPrefix(value, "/") {
		return errors.New("a path starts with /")
	}
	if strings.ContainsAny(value, " \t\"") {
		return errors.New("a path holds no spaces or quotes")
	}
	if strings.Count(value, "{") != strings.Count(value, "}") {
		return errors.New("every {wildcard} needs its closing brace")
	}
	return nil
}

// validateHandlerName keeps the name usable as both an identifier and a file
// stem. An empty answer keeps the one derived from the route.
func validateHandlerName(value string) error {
	if value == "" {
		return nil
	}
	for index, r := range value {
		if unicode.IsLetter(r) || (index > 0 && unicode.IsDigit(r)) {
			continue
		}
		return errors.New("a Go identifier holds letters and digits only")
	}
	if unicode.IsUpper([]rune(value)[0]) {
		return errors.New("a handler stays unexported; start with a lowercase letter")
	}
	return nil
}
