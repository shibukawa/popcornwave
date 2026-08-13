// Package pagesfixture_test serves the committed page tree, so the generated
// registry is exercised as a running router rather than only compared as text.
//
// The tree itself is internal/pagesfixture/pages: a root page under a layout, a
// dynamic route whose typed Load feeds the page component, and a server action
// beside it. internal/pwcli owns generating it.
package pagesfixture_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/internal/pagesfixture/pages"
	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/pwruntime"
	"github.com/shibukawa/popcornwave/testutil"
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

// fixtureRequest is a request carrying a CSRF secret, which is what the session
// middleware supplies in a running application and what this fixture has no
// middleware stack to install.
//
// A page whose template declares a form action renders an unsafe form, and the
// module fails such a render outright rather than emitting an empty token. So
// the secret is not decoration here: without it the users page answers 500 and
// the reason reaches the log only.
func fixtureRequest(method, path string) *http.Request {
	request := httptest.NewRequest(method, path, nil)
	return request.WithContext(pwruntime.WithCSRFSecret(request.Context(), fixtureCSRFSecret))
}

// A real secret rather than any string: the token derivation decodes it and
// yields nothing when it cannot, so a placeholder would fail the render exactly
// as an absent one does, and say the same thing about it.
var fixtureCSRFSecret = func() string {
	secret, err := pwruntime.NewCSRFSecret(nil)
	if err != nil {
		panic(err)
	}
	return secret
}()

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
		{"/users/42?page=3", []string{
			"<p>page 3</p>",
			// A client handler is lowered away and named on the element, so the
			// runtime finds every bound element with one indexed query.
			`<h1 data-tb-on="click:highlight">user 42</h1>`,
			// The form lowering: no action attribute, so a native submit goes to
			// the document URL, which already holds this page's path parameters.
			`<form data-tb-action="/_action/d71506d06c1e/Retire" method="post">`,
			`<input type="hidden" name="_action" value="d71506d06c1e/Retire" />`,
			`name="_csrf"`,
		}},
		// An absent optional query value is not a zero one: Load sees nil and
		// applies its own default.
		{"/users/42", []string{"<p>page 1</p>"}},
	} {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, fixtureRequest(http.MethodGet, testCase.path))
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

// A form action is reachable by a native submit, which is the half of the
// feature that needs no browser runtime at all.
//
// The form carries no action attribute, so the browser posts to the document
// URL — this same path, with its parameters already filled in — and the hidden
// selector is what says which server function ran. Retire writes nothing, so the
// generated dispatcher answers the post-redirect-get default rather than a body.
func TestPagesServeFormAction(t *testing.T) {
	mux := fixtureMux(t)
	body := strings.NewReader(url.Values{"_action": {"d71506d06c1e/Retire"}, "reason": {"left"}}.Encode())
	request := httptest.NewRequest(http.MethodPost, "/users/42", body)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status %d, want 303\n%s", recorder.Code, recorder.Body.String())
	}
	// Back to the page it was submitted from, so a reload does not resubmit.
	if location := recorder.Header().Get("Location"); location != "/users/42" {
		t.Errorf("Location is %q, want the page it came from", location)
	}
}

// A selector no server function on this page matches is refused rather than
// dispatched, because the page POST is one route serving several handlers and
// the selector is the only thing separating them.
func TestPagesServeFormActionRejectsUnknownSelector(t *testing.T) {
	mux := fixtureMux(t)
	body := strings.NewReader(url.Values{"_action": {"000000000000/Nothing"}}.Encode())
	request := httptest.NewRequest(http.MethodPost, "/users/42", body)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400\n%s", recorder.Code, recorder.Body.String())
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

// The redraw endpoint, from the template annotation to the served region.
//
// Nothing in this package registers Card: the .pw.html source carries
// @reloadable, generation derived an init from that, and linking the package is
// what put it in the runtime registry. Serving it here is the only assertion
// that covers the whole chain, because every link in it is invisible in Go
// source a person wrote.
//
// It runs the real server rather than the mux, so the request passes through the
// same middleware an ordinary page request does — which is the point: the redraw
// is answered at the page's own URL and inherits everything that guards it.
func TestPagesServeRedrawsAnAnnotatedComponent(t *testing.T) {
	server := testutil.TestRun(t, fixtureMux(t), func(config *testutil.Config) {
		testutil.Update[pw.HTMLConfig](config, func(html *pw.HTMLConfig) {
			html.Update.Enabled = true
			html.Update.ValidatorKey = "fixture-validator-key"
		})
		// The fixture registers no public filesystem, and startup refuses the
		// endpoint without one.
		testutil.Update[pw.ServerConfig](config, func(server *pw.ServerConfig) {
			server.Public.Enabled = false
		})
	})
	defer server.Close()

	// The browser asks the page it is on, naming the component in headers. That
	// is what puts the redraw behind the page's own authentication instead of a
	// reserved path with a protection rule of its own.
	request, err := http.NewRequest(http.MethodGet, server.URL+"/?page=7", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Pw-Render", "redraw")
	request.Header.Set("Pw-Kind", pages.CardKind)
	request.Header.Set("Pw-Instance", "card-1")
	// A redraw from another build is left to the page, so carrying the identity
	// is part of what the endpoint is being asked here.
	request.Header.Set("Pw-Build", pw.UpdateBuildID())
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("redraw: status %d\n%s", response.StatusCode, body)
	}
	// The parameter arrived through the generated typed decoder, so a rendered
	// 7 is what separates a real redraw from a page that merely rendered.
	if strings.Contains(string(body), "<h1>home</h1>") {
		t.Fatalf("the whole page was rendered rather than the one region:\n%s", body)
	}
	if !strings.Contains(string(body), "card page 7") {
		t.Errorf("redraw did not render the component:\n%s", body)
	}
}

// The shape a handler writes: name the page, not a list beside it. The set comes
// from what the page's markup can contain, folded at generation, so adding a
// component to the template is the only edit adding it to the surface needs.
//
// It compiles at all only because the page's parameters carry the set, which is
// the property under test here — a page reaching no reloadable component does
// not satisfy the constraint and this call would not build.
func TestRedrawTakesThePageItself(t *testing.T) {
	server := testutil.TestRun(t, pw.NewServeMux(), func(config *testutil.Config) {
		testutil.Update[pw.HTMLConfig](config, func(html *pw.HTMLConfig) {
			html.Update.Enabled = true
			html.Update.ValidatorKey = "fixture-validator-key"
		})
		testutil.Update[pw.ServerConfig](config, func(server *pw.ServerConfig) {
			server.Public.Enabled = false
		})
	})
	defer server.Close()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/?page=3", nil)
	request.Header.Set("Pw-Render", "redraw")
	request.Header.Set("Pw-Kind", pages.CardKind)
	request.Header.Set("Pw-Instance", "card-1")
	request.Header.Set("Pw-Build", pw.UpdateBuildID())
	request = request.WithContext(server.Context())

	if !pw.Redraw(recorder, request, pages.Page) {
		t.Fatal("naming the page did not answer its redraw")
	}
	if body := recorder.Body.String(); !strings.Contains(body, "card page 3") {
		t.Errorf("the redraw did not render the component: %s", body)
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
