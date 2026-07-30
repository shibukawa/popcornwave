package pwcli

import (
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestProject(t *testing.T) (projectState, string) {
	t.Helper()
	root := writeScaffoldedProject(t, initOptions{Name: "fixture", TinyGo: true, Devbox: true, Database: true, Auth: authNone})
	state, err := loadProjectState(root)
	if err != nil {
		t.Fatal(err)
	}
	return state, root
}

func TestNewHandlerWritesASourceAndItsTemplate(t *testing.T) {
	state, root := newTestProject(t)
	options := newOptions{
		Kind: newKindHandler, Package: "handlers", Method: "GET", Path: "/tasks",
		Name: "getTasks", HTML: true, Input: true,
	}
	plan, err := planHandler(state, options)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.apply(root); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(root, "handlers", "getTasks_handler.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`func init() { mux.HandleFunc("GET /tasks", getTasks) }`,
		"pw.Parse[getTasksInput](r)",
		"pw.WriteHTML(w, r, GetTasks(GetTasksParams{Name: input.Name}))",
	} {
		if !strings.Contains(string(source), want) {
			t.Fatalf("handler is missing %q:\n%s", want, source)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "getTasks_handler.go", source, parser.AllErrors); err != nil {
		t.Fatalf("handler is invalid Go: %v\n%s", err, source)
	}
	template, err := os.ReadFile(filepath.Join(root, "handlers", "getTasks.pw.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(template), "export component GetTasks(name: string): html") {
		t.Fatalf("template does not export the component the handler renders:\n%s", template)
	}
	if strings.Contains(string(template), "<!doctype") {
		t.Fatalf("the page duplicates the document shell:\n%s", template)
	}
	// The mux already exists, so nothing about main.go has to change.
	if len(plan.manual) != 0 {
		t.Fatalf("manual steps = %#v", plan.manual)
	}
}

func TestNewHandlerWritesAJSONHandlerWithoutATemplate(t *testing.T) {
	state, root := newTestProject(t)
	plan, err := planHandler(state, newOptions{
		Kind: newKindHandler, Package: "handlers", Method: "POST", Path: "/tasks",
		Name: "postTasks", HTML: false, Input: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.apply(root); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(root, "handlers", "postTasks_handler.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "pw.WriteAPI(w, r, http.StatusOK, PostTasksResponse{") {
		t.Fatalf("handler does not answer with JSON:\n%s", source)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "postTasks_handler.go", source, parser.AllErrors); err != nil {
		t.Fatalf("handler is invalid Go: %v\n%s", err, source)
	}
	if _, err := os.Stat(filepath.Join(root, "handlers", "postTasks.pw.html")); !os.IsNotExist(err) {
		t.Fatalf("a JSON handler got a page template: %v", err)
	}
}

func TestNewHandlerRefusesADuplicateRoute(t *testing.T) {
	state, _ := newTestProject(t)
	// The scaffolded home handler already owns GET /.
	_, err := planHandler(state, newOptions{
		Kind: newKindHandler, Package: "handlers", Method: "GET", Path: "/",
		Name: "getIndex", HTML: true,
	})
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("err = %v, want a duplicate-route refusal", err)
	}
}

func TestNewHandlerRefusesAnExistingFile(t *testing.T) {
	state, root := newTestProject(t)
	options := newOptions{
		Kind: newKindHandler, Package: "handlers", Method: "GET", Path: "/tasks",
		Name: "getTasks", HTML: true,
	}
	plan, err := planHandler(state, options)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.apply(root); err != nil {
		t.Fatal(err)
	}
	// A second run collides with the source the first one wrote.
	options.Path = "/tasks/all"
	plan, err = planHandler(state, options)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.apply(root); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want a conflict", err)
	}
}

// A new package needs its own mux, and wiring it into the entry point is the
// application's call, so it is printed rather than injected.
func TestNewHandlerScaffoldsAMuxForANewPackage(t *testing.T) {
	state, root := newTestProject(t)
	plan, err := planHandler(state, newOptions{
		Kind: newKindHandler, Package: "handlers/admin", Method: "GET", Path: "/dashboard",
		Name: "getDashboard", HTML: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.apply(root); err != nil {
		t.Fatal(err)
	}
	index, err := os.ReadFile(filepath.Join(root, "handlers", "admin", "index.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "func Handlers() *pw.ServeMux") {
		t.Fatalf("the new package has no mux:\n%s", index)
	}
	if len(plan.manual) == 0 || !strings.Contains(plan.manual[0], "handlers/admin") {
		t.Fatalf("manual steps = %#v", plan.manual)
	}
	source, err := os.ReadFile(filepath.Join(root, "handlers", "admin", "getDashboard_handler.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "package admin") {
		t.Fatalf("the handler declares the wrong package:\n%s", source)
	}
}

// A directory the template purpose does not list would take the page template
// and generate nothing from it, so the command stops instead.
func TestNewHandlerRefusesHTMLOutsideTheTemplatePurpose(t *testing.T) {
	root := writeScaffoldedProject(t, initOptions{Name: "fixture", TinyGo: true, Devbox: true, Database: true, Auth: authNone})
	config := filepath.Join(root, "popcornwave.toml")
	source, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	narrowed := strings.Replace(string(source),
		`templates = ["handlers", "templates"]`, `templates = ["templates"]`, 1)
	if err := os.WriteFile(config, []byte(narrowed), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := loadProjectState(root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = planHandler(state, newOptions{
		Kind: newKindHandler, Package: "handlers", Method: "GET", Path: "/tasks",
		Name: "getTasks", HTML: true,
	})
	if err == nil || !strings.Contains(err.Error(), "generate.templates") {
		t.Fatalf("err = %v, want a refusal naming the purpose", err)
	}
}

func TestDefaultHandlerNameFollowsTheRoute(t *testing.T) {
	for _, testcase := range []struct{ method, path, want string }{
		{"GET", "/", "getIndex"},
		{"GET", "/tasks", "getTasks"},
		{"POST", "/tasks/{id}/items", "postTasksIdItems"},
		{"DELETE", "/assets/", "deleteAssets"},
	} {
		if name := defaultHandlerName(testcase.method, testcase.path); name != testcase.want {
			t.Errorf("defaultHandlerName(%q, %q) = %q, want %q", testcase.method, testcase.path, name, testcase.want)
		}
	}
}

func TestHandlerDestinationsStayInsideTheHandlerPurpose(t *testing.T) {
	state, root := newTestProject(t)
	if err := os.MkdirAll(filepath.Join(root, "handlers", "admin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "tools"), 0o755); err != nil {
		t.Fatal(err)
	}
	destinations, err := handlerDestinations(state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(destinations, ",") != "handlers,handlers/admin" {
		t.Fatalf("destinations = %v", destinations)
	}
}

func TestPreselectedPackagePrefersTheWorkingDirectory(t *testing.T) {
	state, root := newTestProject(t)
	if err := os.MkdirAll(filepath.Join(root, "handlers", "admin"), 0o755); err != nil {
		t.Fatal(err)
	}
	destinations, err := handlerDestinations(state)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(root, "handlers", "admin"))
	if selected := preselectedPackage(state, destinations); selected != "handlers/admin" {
		t.Fatalf("selected = %q, want the working directory", selected)
	}
	t.Chdir(root)
	if selected := preselectedPackage(state, destinations); selected != "handlers" {
		t.Fatalf("selected = %q, want the first listed destination", selected)
	}
}

func TestValidateRoutePathRejectsUnusablePatterns(t *testing.T) {
	for _, path := range []string{"tasks", "/tasks {id}", `/tasks"`, "/tasks/{id"} {
		if err := validateRoutePath(path); err == nil {
			t.Errorf("validateRoutePath(%q) accepted it", path)
		}
	}
	for _, path := range []string{"/", "/tasks", "/tasks/{id}", "/assets/"} {
		if err := validateRoutePath(path); err != nil {
			t.Errorf("validateRoutePath(%q) = %v", path, err)
		}
	}
}

// TestRunNewWizardOverKeystrokes exercises the real Bubble Tea program.
func TestRunNewWizardOverKeystrokes(t *testing.T) {
	state, root := newTestProject(t)
	t.Chdir(root)
	destinations, err := handlerDestinations(state)
	if err != nil {
		t.Fatal(err)
	}
	// package, method, "/tasks", the derived name, HTML, no input, then review.
	keystrokes := "\r\r/tasks\r\r\r\r\r"
	options, err := runNewWizard(state, destinations,
		newOptions{Kind: newKindHandler, Package: "handlers", Method: "GET", HTML: true},
		tea.WithInput(strings.NewReader(keystrokes)),
		tea.WithOutput(io.Discard),
		tea.WithoutRenderer(),
		tea.WithoutSignalHandler(),
	)
	if err != nil {
		t.Fatal(err)
	}
	want := newOptions{
		Kind: newKindHandler, Package: "handlers", Method: "GET", Path: "/tasks",
		Name: "getTasks", HTML: true,
	}
	if options != want {
		t.Fatalf("options = %#v, want %#v", options, want)
	}
}

func TestNewWizardReviewListsTheFiles(t *testing.T) {
	state, root := newTestProject(t)
	t.Chdir(root)
	destinations, err := handlerDestinations(state)
	if err != nil {
		t.Fatal(err)
	}
	model := newNewWizard(state, destinations, newOptions{
		Kind: newKindHandler, Package: "handlers", Method: "GET", Path: "/tasks",
		Name: "getTasks", HTML: true,
	})
	review := strings.Join(model.plan(model.defaults), "\n")
	for _, want := range []string{
		"create  handlers/getTasks_handler.go",
		"create  handlers/getTasks.pw.html",
	} {
		if !strings.Contains(review, want) {
			t.Fatalf("review is missing %q:\n%s", want, review)
		}
	}
}
