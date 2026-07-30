// Package pagesfixture_test serves the committed page tree, so the generated
// registry is exercised as a running router rather than only compared as text.
//
// The tree itself is internal/pagesfixture/pages: a root page under a layout, a
// dynamic route whose typed Load feeds the page component, and a server action
// beside it. internal/pwcli owns generating it.
package pagesfixture_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pagesfixture/pages"
	"github.com/shibukawa/popcornwave/pw"
)

// fixtureMux registers the tree on a mux and installs a document shell.
//
// The shell is the fixture's own layout bound a second time: a wrapper is a
// wrapper, and reusing one keeps the fixture free of a second template while
// still proving that the registered document ends up outside the route's own
// layouts.
func fixtureMux(t *testing.T) *pw.ServeMux {
	t.Helper()
	mux := pw.NewServeMux()
	pages.Register(mux)
	return mux
}

func TestMain(m *testing.M) {
	pw.RegisterHTMLDocument(pages.BindLayout(pages.LayoutParams{}))
	m.Run()
}

// A directory holding a page template answers its route, with no registration
// written by hand anywhere in this package.
func TestPagesServeDiscoveredRoutes(t *testing.T) {
	mux := fixtureMux(t)
	for _, testCase := range []struct {
		path string
		want []string
	}{
		{"/", []string{"<h1>home</h1>"}},
		{"/users/42?page=3", []string{"<h1>user 42</h1>", "<p>page 3</p>"}},
		// An absent optional query value is not a zero one: Load sees nil and
		// applies its own default.
		{"/users/42", []string{"<p>page 1</p>"}},
	} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, testCase.path, nil))
		body := recorder.Body.String()
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: status %d\n%s", testCase.path, recorder.Code, body)
		}
		for _, want := range testCase.want {
			if !strings.Contains(body, want) {
				t.Errorf("%s: body is missing %q:\n%s", testCase.path, want, body)
			}
		}
		// The registered document wraps the route's own layout, so the same
		// element appears twice and the outer one is the document.
		if strings.Count(body, `<main class="app">`) != 2 {
			t.Errorf("%s: document did not wrap the layout:\n%s", testCase.path, body)
		}
	}
}

// A route with no page is not a route, and a page is GET only.
func TestPagesServeRejectsWhatIsNotAPage(t *testing.T) {
	mux := fixtureMux(t)
	for _, testCase := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/users", http.StatusNotFound},
		{http.MethodGet, "/nothing/here", http.StatusNotFound},
		{http.MethodPost, "/", http.StatusMethodNotAllowed},
	} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(testCase.method, testCase.path, nil))
		if recorder.Code != testCase.want {
			t.Errorf("%s %s: status %d, want %d", testCase.method, testCase.path, recorder.Code, testCase.want)
		}
	}
}

// A URL input that cannot be read is an error rather than a zero value, and it
// is answered through the framework's problem response.
func TestPagesServeRejectsUnreadableInput(t *testing.T) {
	mux := fixtureMux(t)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/users/42?page=x", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400\n%s", recorder.Code, recorder.Body.String())
	}
}

// A server action is an ordinary handler at a generated address, and it reads a
// typed request: the binder it calls is generated because generation runs over
// the packages of the tree, not only over its templates.
func TestPagesServeServerAction(t *testing.T) {
	mux := fixtureMux(t)
	endpoint := ""
	for _, action := range pages.Actions {
		if action.Handler == "Rename" {
			endpoint = action.Path
		}
	}
	if endpoint == "" {
		t.Fatalf("the action table does not list Rename: %+v", pages.Actions)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader(`{"name":"new"}`))
	request.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"new"`) {
		t.Errorf("action: status %d\n%s", recorder.Code, recorder.Body.String())
	}

	// The check tag is only enforced if the generated binder is the one reading
	// the body, which is what makes this the proof rather than the status above.
	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("missing required field: status %d, want 400\n%s", recorder.Code, recorder.Body.String())
	}
}

// The route table reports what the filesystem knows, which is the material a
// sitemap or a route inspector is built from.
func TestPagesRouteTable(t *testing.T) {
	want := map[string]string{
		"GET /{$}":        "",
		"GET /users/{id}": "users/id_",
	}
	if len(pages.Routes) != len(want) {
		t.Fatalf("route table: %+v", pages.Routes)
	}
	for _, route := range pages.Routes {
		directory, ok := want[route.Pattern]
		if !ok {
			t.Errorf("unexpected route %q", route.Pattern)
			continue
		}
		if route.Dir != directory {
			t.Errorf("%s: directory %q, want %q", route.Pattern, route.Dir, directory)
		}
	}
}
