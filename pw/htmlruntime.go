package pw

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

//go:embed boundary.js
var boundarySource string

//go:embed update.js
var updateSource string

//go:embed updateboot.js
var updateBootSource string

// boundaryRuntimeScript is the framework's own half of the browser runtime: it
// applies streamed await boundaries and reads the live delivery stream.
//
// The framing it reacts to is written by writeBoundaryCompletion, and the two
// are one design: htmlbind emits the placeholder and the settled fragment, and
// everything about how a fragment travels and lands belongs here.
//
// The trigger is the trailing marker, never the template element. An HTML
// parser inserts an element when it reads its start tag, so code reacting to
// the template's insertion can read one whose content has not arrived yet and
// replace the placeholder with nothing, losing the fallback along with the
// result. Because the marker follows the closing template tag in the byte
// stream, it cannot exist before its template is complete, however a proxy, a
// TLS record, or a compressing encoder split the bytes.
//
// It lives in boundary.js rather than in a Go string literal so a formatter, a
// linter, and an editor can all read it, which matters more now that the served
// asset is this file plus a module several times its size.
var boundaryRuntimeScript = boundarySource

// mergedRuntimeScript is the one asset a document loads.
//
// Every byte of it is this framework's. The dependency's own client used to be
// concatenated above these files, and it was removed because the protocol's
// names, its endpoints, and its server are all this framework's: a browser half
// belonging to a dependency meant a change this framework could make alone —
// where a redraw is addressed, most concretely — needed a coordinated release.
// system:tinybind v0.3.5 published the wire contract and made its own asset
// opt-in, which is what made writing this one an ordinary piece of work.
//
// The gain beyond addressing is that there is now one apply core rather than
// two. The merged asset used to carry the module's swap and this framework's
// side by side, with no channel between them and nothing making them agree.
//
// Order is load-bearing. boundary.js defines custom elements at module scope
// and the parser may upgrade one during the define call, so it comes first and
// everything it can reach at that moment is declared inside it. update.js
// installs nothing, and the bootstrap below builds the single instance.
var mergedRuntimeScript = sync.OnceValue(func() string {
	return boundarySource + "\n" + updateSource + "\n" + updateBootSource
})

// frameworkScriptPrefix is reserved for framework-owned browser assets. It is a
// fixed absolute path rather than a subtree of the configurable public mount,
// because these belong to the framework rather than to the application, and
// because the document shell has to be able to name one without reading
// configuration.
const frameworkScriptPrefix = "/_pw/"

const boundaryRuntimeName = "popcornwave-runtime.js"

// scriptRevision digests the script set so a changed dependency changes every
// URL. Deriving it from the bytes rather than from a release constant means an
// htmlbind upgrade that changes the runtime cannot ship under a URL a browser
// already cached forever.
var scriptRevision = sync.OnceValue(func() string {
	sum := sha256.Sum256([]byte(mergedRuntimeScript()))
	return hex.EncodeToString(sum[:])[:16]
})

// RuntimeScriptURL is the absolute path of the boundary runtime module. A
// document template names it through a declared external function rather than
// as a literal, so the template text survives an upgrade that moves the URL.
func RuntimeScriptURL() string {
	return frameworkScriptPrefix + scriptRevision() + "/" + boundaryRuntimeName
}

// serveFrameworkScript answers a framework asset request and reports whether it
// handled the request.
func serveFrameworkScript(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != RuntimeScriptURL() {
		// Only this one URL is claimed. The prefix holds more than the asset —
		// the redraw endpoint lives here too — so answering for the whole
		// namespace here would swallow every route that arrived after this one.
		// serveReservedPath keeps the namespace closed, below all of them.
		return false
	}
	if !operationalMethod(w, r) {
		return true
	}
	header := w.Header()
	header.Set("Content-Type", "text/javascript; charset=utf-8")
	// The revision segment never serves different bytes, so this is genuinely
	// immutable rather than merely long-lived.
	header.Set("Cache-Control", "public, max-age=31536000, immutable")
	header.Set("Content-Length", strconv.Itoa(len(mergedRuntimeScript())))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return true
	}
	_, _ = w.Write([]byte(mergedRuntimeScript()))
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
	if !strings.HasPrefix(r.URL.Path, frameworkScriptPrefix) {
		return false
	}
	http.NotFound(w, r)
	return true
}
