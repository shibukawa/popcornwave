package pw

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// boundaryRuntimeScript applies streamed await boundaries in the browser.
//
// The framing this reacts to is written by writeBoundaryCompletion, and the two
// are one design: htmlbind emits the placeholder and the settled fragment, and
// everything about how a fragment travels and lands belongs here.
//
// The trigger is the trailing marker, never the template element. An HTML
// parser inserts an element when it reads its start tag, so code reacting to
// the template's insertion can read one whose content has not arrived yet and
// replace the placeholder with nothing, losing the fallback along with the
// result. Because the marker follows the closing template tag in the byte
// stream, it cannot exist before its template is complete, however a proxy, a
// TLS record, or a compressing encoder split the bytes. This is invisible in
// development, where a small completion arrives in one chunk.
const boundaryRuntimeScript = `// Popcorn Wave boundary runtime.

// replaced records that the document has been given up on. A replacement is
// terminal: the page it patched no longer exists, so a completion arriving
// after it belongs to a document that is gone.
let replaced = false;

export function applyBoundary(id, fragment) {
	if (replaced) return false;
	const placeholder = document.getElementById(id);
	if (!placeholder) return false;
	placeholder.replaceWith(fragment);
	return true;
}

export function applyHTML(id, html) {
	const holder = document.createElement("template");
	holder.innerHTML = html;
	return applyBoundary(id, holder.content);
}

export function replaceDocument(fragment) {
	replaced = true;
	document.body.replaceChildren(fragment);
}

customElements.define("tb-apply", class extends HTMLElement {
	connectedCallback() {
		const id = this.getAttribute("for");
		this.remove();
		if (!id) return;
		const quoted = id.replace(/["\\]/g, "\\$&");
		const template = document.querySelector('template[data-tb-boundary="' + quoted + '"]');
		if (!template) return;
		applyBoundary(id, template.content);
		template.remove();
	}
});

customElements.define("tb-apply-document", class extends HTMLElement {
	connectedCallback() {
		this.remove();
		const template = document.querySelector("template[data-tb-document]");
		if (!template) return;
		// replaceChildren drops the template along with the page it replaces,
		// and every pending placeholder with it.
		replaceDocument(template.content);
	}
});
`

// frameworkScriptPrefix is reserved for framework-owned browser assets. It is a
// fixed absolute path rather than a subtree of the configurable public mount,
// because these belong to the framework rather than to the application, and
// because the document shell has to be able to name one without reading
// configuration.
const frameworkScriptPrefix = "/_pw/"

const boundaryRuntimeName = "boundary.js"

// scriptRevision digests the script set so a changed dependency changes every
// URL. Deriving it from the bytes rather than from a release constant means an
// htmlbind upgrade that changes the runtime cannot ship under a URL a browser
// already cached forever.
var scriptRevision = sync.OnceValue(func() string {
	sum := sha256.Sum256([]byte(boundaryRuntimeScript))
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
	header.Set("Content-Length", strconv.Itoa(len(boundaryRuntimeScript)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return true
	}
	_, _ = w.Write([]byte(boundaryRuntimeScript))
	return true
}
