package pwcli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A route path is what an operator types; the directories that serve it are
// what the filesystem holds. The two spellings differ because a route directory
// is also a Go package.
func TestParsePagePath(t *testing.T) {
	for _, testcase := range []struct {
		path string
		want []string
	}{
		{"/", nil},
		{"/about", []string{"about"}},
		{"users/{id}", []string{"users", "id_"}},
		{"/files/{rest...}", []string{"files", "rest__"}},
	} {
		segments, err := parsePagePath(testcase.path)
		if err != nil {
			t.Errorf("%s: %v", testcase.path, err)
			continue
		}
		var directories []string
		for _, segment := range segments {
			directories = append(directories, segment.Directory)
		}
		if strings.Join(directories, "/") != strings.Join(testcase.want, "/") {
			t.Errorf("%s: directories = %v, want %v", testcase.path, directories, testcase.want)
		}
	}
}

// A directory name the Go toolchain rejects breaks the build of every package
// in the module, so it is refused here with the reason rather than written.
func TestParsePagePathRejectsWhatGoRejects(t *testing.T) {
	for _, path := range []string{
		"/users/[id]",
		"/users/(group)/x",
		"/users/{}",
		"/files/{rest...}/more",
		"/a//b",
	} {
		if _, err := parsePagePath(path); err == nil {
			t.Errorf("%s was accepted", path)
		}
	}
}

func pageProjectState(t *testing.T) projectState {
	t.Helper()
	root := writePageTreeFixture(t)
	// loadProjectState reads the environment configuration a capability would
	// write into; a page writes none, but the state is shared with pw add.
	writeTestFile(t, filepath.Join(root, "config.dev.toml"), "[server]\naddr = \":8080\"\n")
	state, err := loadProjectState(root)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

// Adding a page writes a directory and a template. Nothing registers it, so
// unlike a handler there is no mux to create and nothing to wire up by hand.
func TestPlanPageWritesTheRouteDirectory(t *testing.T) {
	state := pageProjectState(t)
	plan, err := planPage(state, newOptions{
		Kind: newKindPage, Package: "pages", Path: "/tasks/{id}", Rung: pageRungTemplate,
	})
	if err != nil {
		t.Fatal(err)
	}
	source, wrote := plan.creates["pages/tasks/id_/page.pw.html"]
	if !wrote {
		t.Fatalf("the page template was not planned: %v", plan.creates)
	}
	// The leading parameters of a page are its dynamic segments, in route order.
	if !strings.Contains(source, "export component Page(id: string): html") {
		t.Errorf("the page does not declare its route input:\n%s", source)
	}
	if _, wrote := plan.creates["pages/tasks/id_/page.go"]; wrote {
		t.Error("the template rung wrote a Go file")
	}
	if len(plan.manual) != 0 {
		t.Errorf("a page needs no manual step: %v", plan.manual)
	}
	if !plan.generate {
		t.Error("the plan does not regenerate, so the route would not be registered")
	}
}

// The loader rung scaffolds a page that fetches its own data: the loader is
// declared in the template, bound there, and takes the route's input. Nothing
// generated calls it, which is what requirement:explicit-page-loading moved.
func TestPlanPageLoaderRungWritesADeclaredLoader(t *testing.T) {
	state := pageProjectState(t)
	plan, err := planPage(state, newOptions{
		Kind: newKindPage, Package: "pages", Path: "/tasks/{id}", Rung: pageRungLoader,
	})
	if err != nil {
		t.Fatal(err)
	}
	load := plan.creates["pages/tasks/id_/page.go"]
	// The trailing error is the half that lets a loader answer 404 before the
	// response commits, so a scaffold without it teaches the wrong shape.
	if !strings.Contains(load, "func LoadGreeting(id string) (string, error)") {
		t.Errorf("the loader does not take the route input and an error:\n%s", load)
	}
	page := plan.creates["pages/tasks/id_/page.pw.html"]
	for _, want := range []string{
		"external LoadGreeting(id: string): string",
		"export component Page(id: string): html",
		"{val greeting = LoadGreeting(id)}",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not contain %q:\n%s", want, page)
		}
	}
}

// The handler rung owns its response, which means composing the chain itself:
// a handwritten Load cannot be called by the generated composer above it.
func TestPlanPageHandlerRungComposesItsOwnChain(t *testing.T) {
	state := pageProjectState(t)
	plan, err := planPage(state, newOptions{
		Kind: newKindPage, Package: "pages", Path: "/tasks", Rung: pageRungHandler,
	})
	if err != nil {
		t.Fatal(err)
	}
	load := plan.creates["pages/tasks/page.go"]
	for _, want := range []string{
		"func Load(w http.ResponseWriter, r *http.Request)",
		"Render(w, r, route, PageParams{})",
		"pw.WriteProblem(w, r, err)",
	} {
		if !strings.Contains(load, want) {
			t.Errorf("the handler rung is missing %q:\n%s", want, load)
		}
	}
}

func TestPlanPageRefusesAnExistingRoute(t *testing.T) {
	state := pageProjectState(t)
	if _, err := planPage(state, newOptions{
		Kind: newKindPage, Package: "pages", Path: "/users/{id}", Rung: pageRungTemplate,
	}); err == nil {
		t.Error("an existing route was overwritten")
	}
}

// A tree that already has a layout is not offered another one, and a tree
// without one gets it beside the root.
func TestPlanPageWritesALayoutOnlyWhenAsked(t *testing.T) {
	state := pageProjectState(t)
	plan, err := planPage(state, newOptions{
		Kind: newKindPage, Package: "pages", Path: "/tasks", Rung: pageRungTemplate, Layout: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The fixture tree already has one, so asking cannot overwrite it.
	if _, wrote := plan.creates["pages/layout.pw.html"]; wrote {
		t.Error("an existing layout was rewritten")
	}
	if err := os.Remove(filepath.Join(state.root, "pages", "layout.pw.html")); err != nil {
		t.Fatal(err)
	}
	if hasPageLayout(state, "pages") {
		t.Error("a tree with no layout reports one")
	}
	plan, err = planPage(state, newOptions{
		Kind: newKindPage, Package: "pages", Path: "/tasks", Rung: pageRungTemplate, Layout: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	layout := plan.creates["pages/layout.pw.html"]
	if !strings.Contains(layout, "children: html") {
		t.Errorf("the layout does not declare the slot parameter:\n%s", layout)
	}
}

// A capability declined at init is not a decision the project is stuck with:
// adding the page tree afterwards has to reach the file state init would have
// written, and open the purpose that makes it generate.
func TestAddPagesReachesTheScaffoldedState(t *testing.T) {
	root := declinedProject(t)
	state, err := loadProjectState(root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planCapability(state, addOptions{Capability: capabilityDiscovered})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.apply(root); err != nil {
		t.Fatal(err)
	}

	// declinedProject carries the registered tree, so adding the discovered one
	// makes it a both-router project. That is the state init would have written
	// for the both answer, and it is what the starter page has to describe: the
	// page names the trees the project actually has.
	scaffolded := scaffoldFiles(initOptions{Name: "fixture", Router: routerBoth, TinyGo: true, Auth: authNone})
	for path, want := range scaffolded {
		if !strings.HasPrefix(path, "pages/") {
			continue
		}
		added, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if string(added) != want {
			t.Errorf("%s differs from the scaffolded file:\n%s", path, added)
		}
	}

	// The tree and the purpose that reads it are written together: a tree no
	// purpose lists is a directory nothing generates from.
	reloaded, err := loadProjectState(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.config.Generate.Pages; len(got) != 1 || got[0] != "pages" {
		t.Fatalf("generate.pages = %v", got)
	}
	if evidence, installed, err := reloaded.carries(capabilityDiscovered); err != nil {
		t.Fatal(err)
	} else if !installed {
		t.Errorf("the installed capability is not detected; evidence = %q", evidence)
	}
	// Nothing registers a page, but something has to call Register, and the
	// entry point is application-owned.
	if len(plan.manual) == 0 || !strings.Contains(strings.Join(plan.manual, "\n"), "pages.Register") {
		t.Errorf("the wiring step was not printed: %v", plan.manual)
	}
}

// The other direction of the same catalog entry: a project that started as a
// website gains the registered router, which is what makes the init question a
// starting point rather than a mode.
func TestAddHandlersIntoAPagesOnlyProject(t *testing.T) {
	root := writeScaffoldedProject(t, initOptions{Name: "fixture", Router: routerDiscovered, TinyGo: true, Auth: authNone})
	state, err := loadProjectState(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, installed, err := state.carries(capabilityRegistered); err != nil {
		t.Fatal(err)
	} else if installed {
		t.Fatal("a pages-only project reports the handler tree as installed")
	}

	plan, err := planCapability(state, addOptions{Capability: capabilityRegistered})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.apply(root); err != nil {
		t.Fatal(err)
	}
	reloaded, err := loadProjectState(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.config.Generate.Handlers; len(got) != 1 || got[0] != "handlers" {
		t.Fatalf("generate.handlers = %v", got)
	}
	// A page template sits beside the handler that renders it, so the template
	// purpose has to reach the new package too.
	if !slices.Contains(reloaded.config.Generate.Templates, "handlers") {
		t.Errorf("generate.templates does not reach the handler package: %v", reloaded.config.Generate.Templates)
	}
	// The page tree it started with is untouched.
	if got := reloaded.config.Generate.Pages; len(got) != 1 || got[0] != "pages" {
		t.Errorf("adding a handler tree disturbed the page tree: %v", got)
	}
	if _, err := os.Stat(filepath.Join(root, "handlers", "index.go")); err != nil {
		t.Errorf("the handler package has no mux: %v", err)
	}
}
