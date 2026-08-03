package pw

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/shibukawa/tinybind-go/htmlupdate"
)

//go:embed boundary.js
var boundarySource string

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
// Two runtimes on one page would mean two boundary id spaces, two build
// identities, and two script tags with nothing deciding which owns a region, so
// the module's half is composed in rather than served beside this one. Its
// bytes come from the pinned dependency, not from a copy, so an upgrade that
// changes them changes this asset and its revision with it.
//
// Order is load-bearing. The module's half registers the factory; the bootstrap
// below it builds the instance. The module's own self-instantiation reads
// document.currentScript, which is null in a module script, so it does nothing
// here and cannot produce a second instance.
var mergedRuntimeScript = sync.OnceValue(func() string {
	return string(htmlupdate.RuntimeSource()) + "\n" + boundarySource + "\n" + updateBootSource
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
	if !strings.HasPrefix(r.URL.Path, frameworkScriptPrefix) {
		return false
	}
	if r.URL.Path != RuntimeScriptURL() {
		http.NotFound(w, r)
		return true
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
