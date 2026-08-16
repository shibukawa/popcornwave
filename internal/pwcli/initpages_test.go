package pwcli

import (
	"strings"
	"testing"
)

func routerInitOptions(router string) initOptions {
	options := defaultInitOptions()
	options.Name = "demo"
	options.Router = router
	return options
}

// The default answer is the shape every project scaffolded before page trees
// existed has, so a scripted run that never heard of the question keeps working.
func TestScaffoldDefaultRouterWritesOnlyTheRegisteredTree(t *testing.T) {
	files := scaffoldFiles(routerInitOptions(""))
	if _, wrote := files["handlers/home_handler.go"]; !wrote {
		t.Error("the default scaffold has no handler")
	}
	for path := range files {
		if strings.HasPrefix(path, "pages/") {
			t.Errorf("the default scaffold wrote %s", path)
		}
	}
	project := files["popcornwave.toml"]
	if !strings.Contains(project, "pages = []") {
		t.Errorf("generate.pages is not the empty list:\n%s", project)
	}
	if !strings.Contains(files["cmd/demo/main.go"], "pw.Run(context.Background(), handlers.Handlers())") {
		t.Errorf("entry point changed:\n%s", files["cmd/demo/main.go"])
	}
}

// A website answer writes a page tree and no handler package, so the project
// holds exactly the tree it will add pages to.
func TestScaffoldDiscoveredRouterWritesOnlyTheTree(t *testing.T) {
	files := scaffoldFiles(routerInitOptions(routerDiscovered))
	for _, path := range []string{
		"pages/layout.pw.html",
		"pages/page.pw.html",
		"pages/greet/name_/page.pw.html",
		"pages/greet/name_/page.go",
	} {
		if _, wrote := files[path]; !wrote {
			t.Errorf("the page tree scaffold is missing %s", path)
		}
	}
	for path := range files {
		if strings.HasPrefix(path, "handlers/") {
			t.Errorf("a pages-only scaffold wrote %s", path)
		}
	}

	project := files["popcornwave.toml"]
	if !strings.Contains(project, `pages = ["pages"]`) {
		t.Errorf("generate.pages does not name the tree:\n%s", project)
	}
	if !strings.Contains(project, "handlers = []") {
		t.Errorf("generate.handlers is not empty:\n%s", project)
	}
	// The tree run compiles the page templates, so listing the root under the
	// template purpose would generate one output twice.
	if !strings.Contains(project, `templates = ["templates"]`) {
		t.Errorf("generate.templates does not hold only the shared templates:\n%s", project)
	}

	main := files["cmd/demo/main.go"]
	for _, want := range []string{`"demo/pages"`, "mux := pw.NewServeMux()", "pages.Register(mux)"} {
		if !strings.Contains(main, want) {
			t.Errorf("entry point is missing %q:\n%s", want, main)
		}
	}
	if strings.Contains(main, "handlers") {
		t.Errorf("a pages-only entry point still names the handler package:\n%s", main)
	}

	// The document shell and the error pages serve both routers, so they are
	// written whichever one the project took.
	for _, path := range []string{"templates/document.pw.html", "templates/404.pw.html"} {
		if _, wrote := files[path]; !wrote {
			t.Errorf("the shared %s was not written", path)
		}
	}
}

// Both trees coexist on one mux, which is the whole point of the third answer.
func TestScaffoldBothRoutersShareOneMux(t *testing.T) {
	files := scaffoldFiles(routerInitOptions(routerBoth))
	if _, wrote := files["handlers/home_handler.go"]; !wrote {
		t.Error("the both answer has no handler")
	}
	if _, wrote := files["pages/page.pw.html"]; !wrote {
		t.Error("the both answer has no page tree")
	}
	project := files["popcornwave.toml"]
	if !strings.Contains(project, `handlers = ["handlers"]`) || !strings.Contains(project, `pages = ["pages"]`) {
		t.Errorf("both purposes are not listed:\n%s", project)
	}
	main := files["cmd/demo/main.go"]
	if !strings.Contains(main, "mux := handlers.Handlers()") || !strings.Contains(main, "pages.Register(mux)") {
		t.Errorf("the two routers do not share one mux:\n%s", main)
	}
}

// The page tree scaffold has to survive its own rules: a layout declares the
// slot the chain binds, and a dynamic directory is spelled with an underscore
// because it is also a Go package.
func TestPageTreeScaffoldFollowsTheTreeRules(t *testing.T) {
	files := pageTreeScaffold(routerInitOptions(routerDiscovered), defaultDiscoveredDir)
	if layout := files["pages/layout.pw.html"]; !strings.Contains(layout, "children: html") {
		t.Errorf("the layout does not declare the slot parameter:\n%s", layout)
	}
	if _, wrote := files["pages/greet/name_/page.pw.html"]; !wrote {
		t.Error("the dynamic route is not spelled with a trailing underscore")
	}
	if load := files["pages/greet/name_/page.go"]; !strings.Contains(load, "func LoadGreeting(name string) (string, error)") {
		t.Errorf("the loader changed shape:\n%s", load)
	}
	// The template is what names the loader and binds its result, since a page
	// has no entry point of its own any more.
	page := files["pages/greet/name_/page.pw.html"]
	for _, want := range []string{
		"external LoadGreeting(name: string): string",
		"export component Page(name: string): html",
		"{val greeting = LoadGreeting(name)}",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the scaffolded page is missing %q:\n%s", want, page)
		}
	}
}

func TestParseInitArgsRouter(t *testing.T) {
	for _, testcase := range []struct {
		arg  string
		want string
	}{
		{"--router=registered", routerRegistered},
		{"--router=discovered", routerDiscovered},
		{"--router=both", routerBoth},
	} {
		options, err := parseInitArgs([]string{"demo", testcase.arg})
		if err != nil {
			t.Fatal(err)
		}
		if options.Router != testcase.want {
			t.Errorf("%s: router = %q, want %q", testcase.arg, options.Router, testcase.want)
		}
	}
	// The directory a router reads is a configuration value, so it is not an
	// answer here: pages is a folder name, not the name of a router.
	if _, err := parseInitArgs([]string{"demo", "--router=pages"}); err == nil {
		t.Error("a folder name was accepted as a router answer")
	}
}
