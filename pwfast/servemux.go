package pwfast

import (
	"fmt"
	"strings"

	"github.com/shibukawa/tinygodriver/fasthttp"
	router "github.com/shibukawa/tinygodriver/fasthttprouter"
)

// ServeMux routes Go 1.22 patterns on this transport.
//
// It is the same name and the same patterns as the other half, over a trie
// router instead of net/http's matcher. An application writes one route table
// and both builds register it, which is the point: a route table is the part of
// an application least willing to be written twice.
//
// What it does is translate at registration. The pattern syntax is close enough
// that most of the translation is nothing — a named parameter is spelled the
// same in both — and the rest is three rewrites and two refusals, each of which
// is documented on the function that performs it.
type ServeMux struct {
	router *router.Router
}

// NewServeMux returns a mux whose behaviour matches Go 1.22's.
//
// The four flags are set rather than left at the router's defaults, because the
// defaults are that router's opinion and the contract here is the other
// transport's behaviour.
func NewServeMux() *ServeMux {
	trie := router.New()
	// Go redirects /foo to /foo/ when only the subtree is registered, and back.
	trie.RedirectTrailingSlash = true
	// Off deliberately. This flag does two things, and only one of them matches
	// Go: it cleans ../ and // out of a path, which Go also does, and it then
	// retries the lookup case-insensitively, which Go never does. Leaving it on
	// would make /Admin reach a handler registered for /admin, so a route table
	// that reads as case-sensitive would not be, and that is the kind of
	// difference an authorization check is written on top of.
	trie.RedirectFixedPath = false
	// Go 1.22 answers 405 with an Allow header when the path matches and the
	// method does not.
	trie.HandleMethodNotAllowed = true
	// Go does not answer OPTIONS by itself, so neither does this.
	trie.HandleOPTIONS = false
	return &ServeMux{router: trie}
}

// Handle registers a handler for one Go 1.22 pattern.
func (m *ServeMux) Handle(pattern string, handler fasthttp.RequestHandler) {
	method, path := translatePattern(pattern)
	handler = recordingRoute(pattern, handler)
	if method == "" {
		m.router.ANY(path, handler)
		return
	}
	m.router.Handle(method, path, handler)
}

// recordingRoute writes the matched pattern onto the request value, so a metric
// is keyed by the route rather than by the path.
//
// This transport can do at registration what the other half cannot: the mux is
// this framework's own type, so every handler it dispatches to reports its
// pattern without the handler being asked. On net/http the mux is the standard
// library's and the pattern has to travel back up on the response writer.
//
// The cost is one closure per registration and one user value per request.
func recordingRoute(pattern string, handler fasthttp.RequestHandler) fasthttp.RequestHandler {
	if pattern == "" || handler == nil {
		return handler
	}
	return func(r *fasthttp.RequestCtx) {
		r.SetUserValue(routeUserValueKey{}, pattern)
		handler(r)
	}
}

// HandleFunc is Handle for a bare function, and is the method generated route
// registration calls.
func (m *ServeMux) HandleFunc(pattern string, handler func(*fasthttp.RequestCtx)) {
	m.Handle(pattern, handler)
}

// Handler serves one request, and is the value passed to a server.
//
// It is a method rather than the mux itself being the handler because
// fasthttp's handler is a function type rather than an interface, so there is
// nothing for a struct to implement.
func (m *ServeMux) Handler(ctx *fasthttp.RequestCtx) {
	m.router.Handler(ctx)
}

// subtreeParameter names the catch-all standing in for a Go subtree pattern. It
// is stored as a path value the way any wildcard is, so the name is one an
// application is unlikely to have chosen for a parameter of its own.
const subtreeParameter = "pwSubtree"

// translatePattern converts a Go 1.22 pattern into the router's method and
// path, and refuses the two shapes that have no translation.
//
// The three rewrites:
//
//   - {name...} becomes {name:*}, the same catch-all under another spelling.
//   - {$} is dropped. It exists in Go to opt out of subtree matching, and a
//     trie is exact already, so the marker has no counterpart because it needs
//     none. This one matters more than it sounds: generated page trees register
//     "GET /{$}" for the root, so the first route of every project takes it.
//   - A pattern ending in / is a subtree in Go and an exact path in a trie, so
//     it gains a catch-all. The catch-all also matches the directory itself,
//     which is what Go's subtree pattern does.
//
// The two refusals are shapes a trie cannot express, and they panic the way
// net/http panics on a pattern it cannot parse — at registration, naming the
// pattern, before any request reaches it.
func translatePattern(pattern string) (method, path string) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		panic("pwfast: empty route pattern")
	}
	if space := strings.IndexByte(pattern, ' '); space >= 0 {
		method, path = pattern[:space], strings.TrimSpace(pattern[space+1:])
	} else {
		path = pattern
	}
	if !strings.HasPrefix(path, "/") {
		// Go matches on host when the pattern carries one before the path. The
		// trie has no host dimension, and silently ignoring the host would make
		// a route registered for one host answer for every host.
		panic(fmt.Sprintf("pwfast: route pattern %q matches on host, which this router cannot do; "+
			"give the fasthttp build its own route table for it", pattern))
	}

	switch {
	case path == "/{$}":
		path = "/"
	case strings.HasSuffix(path, "/{$}"):
		path = strings.TrimSuffix(path, "{$}")
	case strings.HasSuffix(path, "/"):
		path += "{" + subtreeParameter + ":*}"
	}
	if strings.Contains(path, "{$}") {
		// Go allows the marker only as the final segment, so anything left here
		// was already invalid rather than merely untranslatable.
		panic(fmt.Sprintf("pwfast: route pattern %q uses {$} somewhere other than the end", pattern))
	}
	return method, rewriteCatchAll(path)
}

// rewriteCatchAll converts Go's {name...} into the router's {name:*}.
func rewriteCatchAll(path string) string {
	if !strings.Contains(path, "...}") {
		return path
	}
	var out strings.Builder
	for {
		open := strings.IndexByte(path, '{')
		if open < 0 {
			out.WriteString(path)
			return out.String()
		}
		close := strings.IndexByte(path[open:], '}')
		if close < 0 {
			out.WriteString(path)
			return out.String()
		}
		close += open
		name := path[open+1 : close]
		out.WriteString(path[:open])
		if strings.HasSuffix(name, "...") {
			out.WriteString("{" + strings.TrimSuffix(name, "...") + ":*}")
		} else {
			out.WriteString("{" + name + "}")
		}
		path = path[close+1:]
	}
}

// RouteInstaller is what generated route registration installs on.
//
// It exists because the two transports register a route differently and the
// generated file has to name one shape. The other half's routes are registered
// by authored wiring the build tag excludes — a mux built in an init, a
// HandleFunc per handler — and this build has to get them from somewhere. What
// it gets is one function per package, emitted from the same route table the
// other half declares, so the two serve the same addresses.
//
// The method and the path arrive apart because that is how the generated call
// spells them. Everything else about the pattern is Go 1.22's, unchanged, and
// is translated here rather than by the generator: a catch-all, a subtree and
// the {$} marker then mean on this transport exactly what they mean on the
// other, decided in one place.
type RouteInstaller interface {
	Handle(method, path string, handler fasthttp.RequestHandler)
}

// Routes adapts a mux to what generated route registration installs on:
//
//	mux := pwfast.NewServeMux()
//	handlers.RegisterRoutes(pwfast.Routes(mux))
func Routes(mux *ServeMux) RouteInstaller { return muxRoutes{mux: mux} }

type muxRoutes struct{ mux *ServeMux }

func (m muxRoutes) Handle(method, path string, handler fasthttp.RequestHandler) {
	m.mux.Handle(strings.TrimSpace(method+" "+path), handler)
}
