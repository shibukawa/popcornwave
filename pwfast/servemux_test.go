package pwfast

import (
	"strings"
	"testing"

	"github.com/shibukawa/tinybind-go/fasthttpbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// The translation is asserted on its own before anything is served, because a
// pattern that translates wrongly still routes something and the failure would
// read as a routing bug rather than a translation one.
func TestPatternTranslation(t *testing.T) {
	for _, row := range []struct{ pattern, method, path string }{
		{"GET /users/{id}", "GET", "/users/{id}"},
		{"/users/{id}", "", "/users/{id}"},
		{"POST /users", "POST", "/users"},
		// The marker Go uses to opt out of subtree matching. A trie is exact
		// already, so it is dropped rather than translated.
		{"GET /{$}", "GET", "/"},
		{"GET /admin/{$}", "GET", "/admin/"},
		// A Go subtree becomes a catch-all, which also matches the directory.
		{"GET /files/", "GET", "/files/{pwSubtree:*}"},
		{"/", "", "/{pwSubtree:*}"},
		// Same catch-all, different spelling.
		{"GET /static/{rest...}", "GET", "/static/{rest:*}"},
		{"GET /a/{x}/b/{y}", "GET", "/a/{x}/b/{y}"},
	} {
		method, path := translatePattern(row.pattern)
		if method != row.method || path != row.path {
			t.Errorf("%q -> (%q, %q), want (%q, %q)", row.pattern, method, path, row.method, row.path)
		}
	}
}

func TestAHostPatternIsRefusedAtRegistration(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("a host pattern registered without complaint")
		}
		// The refusal has to name the pattern, or it points at the framework
		// rather than at the route that needs a second table.
		if !strings.Contains(recovered.(string), "example.com/admin") {
			t.Errorf("the refusal did not name the pattern: %v", recovered)
		}
	}()
	NewServeMux().HandleFunc("GET example.com/admin", func(*fasthttp.RequestCtx) {})
}

func TestTheRootPatternMatchesOnlyTheRoot(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("GET /{$}", func(r *fasthttp.RequestCtx) { _, _ = r.WriteString("root") })
	mux.HandleFunc("GET /users/{id}", func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString("user " + fasthttpbind.PathValue(r, "id"))
	})

	if _, _, body := serve(t, mux.Handler, "/"); body != "root" {
		t.Errorf("/ served %q", body)
	}
	// The point of {$}: without it "/" would be a subtree and would answer here.
	if status, _, _ := serve(t, mux.Handler, "/nothing-here"); status != fasthttp.StatusNotFound {
		t.Errorf("an unregistered path answered %d rather than 404", status)
	}
}

// The path value has to arrive where the generated decoder reads it, which is
// the module's accessor over the router's own store.
func TestAPathParameterReachesTheModuleAccessor(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("GET /users/{id}", func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString(fasthttpbind.PathValue(r, "id"))
	})
	if _, _, body := serve(t, mux.Handler, "/users/4711"); body != "4711" {
		t.Errorf("path value = %q, want 4711", body)
	}
}

func TestASubtreePatternMatchesTheDirectoryAndBelow(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("GET /files/", func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString("files:" + fasthttpbind.PathValue(r, subtreeParameter))
	})

	// The values are net/http's, checked against it rather than assumed: a
	// {rest...} wildcard there yields "css/site.css" and not "/css/site.css",
	// and an empty string for the directory itself. The router agrees, so a
	// handler reading the value sees the same text on either transport.
	for _, row := range []struct{ target, want string }{
		{"/files/", "files:"},
		{"/files/a.txt", "files:a.txt"},
		{"/files/deep/b.txt", "files:deep/b.txt"},
	} {
		if _, _, body := serve(t, mux.Handler, row.target); body != row.want {
			t.Errorf("%s served %q, want %q", row.target, body, row.want)
		}
	}
}

func TestACatchAllSpelledTheGoWayIsTheSameRoute(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("GET /static/{rest...}", func(r *fasthttp.RequestCtx) {
		_, _ = r.WriteString(fasthttpbind.PathValue(r, "rest"))
	})
	// net/http yields "css/site.css" for this pattern and this request; so does
	// the router.
	if _, _, body := serve(t, mux.Handler, "/static/css/site.css"); body != "css/site.css" {
		t.Errorf("catch-all value = %q, want %q", body, "css/site.css")
	}
}

// Go 1.22 answers 405 when the path matches and the method does not, and this
// is one of the four behaviours set rather than inherited.
func TestAWrongMethodIs405RatherThan404(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("POST /users", func(*fasthttp.RequestCtx) {})
	status, header, _ := serve(t, mux.Handler, "/users")
	if status != fasthttp.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", status)
	}
	if !strings.Contains(header, "Allow:") {
		t.Errorf("405 carried no Allow header:\n%s", header)
	}
}

// Case-insensitive matching is off deliberately: a route table that reads as
// case-sensitive has to be one, because access checks are written on top of it.
func TestRoutingIsCaseSensitive(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("GET /admin", func(r *fasthttp.RequestCtx) { _, _ = r.WriteString("admin") })
	if status, _, body := serve(t, mux.Handler, "/Admin"); status == fasthttp.StatusOK {
		t.Errorf("/Admin reached the /admin handler: %d %q", status, body)
	}
}
