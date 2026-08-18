package pw

import (
	"io"
	"net/http"
	"strconv"

	"github.com/shibukawa/popcornweb/pwbrowser"
)

// The browser runtime is popcornweb/pwbrowser's: the bytes, the module set,
// the revision the URL carries, and which paths the set claims. Both transports
// serve one asset under one URL, because a document naming it is rendered by
// whichever half is running and an application's template names it once.
//
// What is here is this transport's half — reading a path off a request and
// writing the response — plus the development set this build mode adds.

// RuntimeScriptURL is the absolute path of the boundary runtime module. A
// document template names it through a declared external function rather than
// as a literal, so the template text survives an upgrade that moves the URL.
func RuntimeScriptURL() string { return pwbrowser.RuntimeScriptURL() }

// frameworkScriptPrefix is reserved for framework-owned browser assets.
const frameworkScriptPrefix = pwbrowser.Prefix

// init publishes what this build mode adds to the set. A release build adds
// nothing and lands on the same revision the other transport does.
func init() { pwbrowser.Publish(developmentScripts(), developmentImport()) }

// serveFrameworkScript answers a framework asset request and reports whether it
// handled the request.
func serveFrameworkScript(w http.ResponseWriter, r *http.Request) bool {
	source, contentType, ok := pwbrowser.Lookup(r.URL.Path)
	if !ok {
		return false
	}
	if !operationalMethod(w, r) {
		return true
	}
	header := w.Header()
	header.Set("Content-Type", contentType)
	header.Set("Cache-Control", pwbrowser.CacheControl)
	header.Set("Content-Length", strconv.Itoa(len(source)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return true
	}
	_, _ = io.WriteString(w, source)
	return true
}

// serveReservedPath closes the framework namespace.
//
// It runs below every handler that owns something inside the prefix, so an
// unclaimed path there is one no framework route serves: a stale script
// revision, a redraw on a deployment publishing no component, or a probe of the
// namespace. Each answers 404 here rather than reaching application routing,
// which is what keeps one reserved prefix a single routing and access rule
// instead of a hole an application could accidentally serve through.
func serveReservedPath(w http.ResponseWriter, r *http.Request) bool {
	if !pwbrowser.Claims(r.URL.Path) {
		return false
	}
	http.NotFound(w, r)
	return true
}
