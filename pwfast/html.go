package pwfast

import (
	"strings"

	"github.com/shibukawa/popcornweb/pwruntime"
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// htmlContentType is what a rendered document and a rendered fragment both
// carry. It is spelled here rather than read from a negotiation, because these
// entries render HTML by definition; what varies between them is the shell,
// not the media type.
const htmlContentType = "text/html; charset=utf-8"

// WriteHTML renders one generated fragment inside the registered document
// shell, which is the entry an ordinary handler calls.
func WriteHTML(r *fasthttp.RequestCtx, leaf HTMLFragment) {
	WriteHTMLChain(r, pwruntime.RegisteredHTMLDocument(), leaf)
}

// WriteHTMLPage renders a page inside its own wrapper chain and the registered
// document shell, with the document outermost. It is what generated page tree
// code calls: that code knows the ancestor layouts of a route and must not name
// the document, which stays the framework's.
func WriteHTMLPage(r *fasthttp.RequestCtx, wrappers []HTMLWrapper, leaf HTMLFragment, options ...HTMLOption) {
	WriteHTMLChain(r, append(pwruntime.RegisteredHTMLDocument(), wrappers...), leaf, options...)
}

// WriteHTMLChain renders generated wrappers around one leaf.
//
// Nothing commits until the whole chain has rendered, so a failure part way
// through still becomes a problem response rather than a truncated document.
// A chain carrying await boundaries settles them before writing: this half
// renders buffered where the net/http one can stream, which costs time to first
// byte and changes no bytes. Streaming is what the deferred htmlupdate port
// unlocks, since the boundary delivery path holds a flusher.
func WriteHTMLChain(r *fasthttp.RequestCtx, wrappers []HTMLWrapper, leaf HTMLFragment, options ...HTMLOption) {
	// A live subscription arrives on the route's own URL, told apart from a page
	// request by the headers it carries — so this is where it is answered,
	// exactly as it is on the other transport. ServeLive reads that negotiation
	// itself and reports whether it took the request, which keeps the condition
	// in one place rather than in every caller.
	//
	// The handler, the layouts and the binding that produced this chain have
	// already run, which is what makes a reconnect need no continuation: the
	// reconstruction path is the render path.
	if ServeLive(r, wrappers, leaf, options...) {
		return
	}
	var body []byte
	buffer := bytesWriter{buf: &body}
	if err := htmlbind.RenderChain(&buffer, wrappers, leaf, options...); err != nil {
		WriteProblem(r, err)
		return
	}
	varyOnDeclaredAxes(r, htmlbind.MergeVary(wrappers, leaf))
	writeChainCachePolicy(r, wrappers, leaf)
	r.Response.Header.SetContentType(htmlContentType)
	r.SetStatusCode(fasthttp.StatusOK)
	_, _ = r.Write(body)
}

// WriteHTMLFragment renders one fragment with no document around it, for a
// response that replaces a region of a page the client already has.
func WriteHTMLFragment(r *fasthttp.RequestCtx, fragment HTMLFragment) {
	var body []byte
	buffer := bytesWriter{buf: &body}
	if err := htmlbind.Render(&buffer, fragment); err != nil {
		WriteProblem(r, err)
		return
	}
	varyOnDeclaredAxes(r, fragment.Vary())
	// A swap target is markup for the screen it lands on, so it carries the same
	// policy that screen does. There is no chain here to assert otherwise: a
	// wrapper is what can declare a whole document shared, and a fragment answers
	// with no wrapper at all, so an undeclared one is private like everything
	// else undeclared.
	if fragment.IsPrivate() {
		r.Response.Header.Set("Cache-Control", privateCacheControl)
	}
	r.Response.Header.SetContentType(htmlContentType)
	r.SetStatusCode(fasthttp.StatusOK)
	_, _ = r.Write(body)
}

// bytesWriter collects a render before anything is committed.
//
// The request value is itself an io.Writer, and rendering straight into it
// would be one copy cheaper — but it would also commit the response before the
// chain finished validating, which is the property that lets a failed render
// still answer with a problem instead of a half-written page.
type bytesWriter struct{ buf *[]byte }

func (w *bytesWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

// privateCacheControl is what a response says when the markup it carries
// belongs to one reader. It is the other transport's word for word, because a
// page that is per-reader is per-reader whichever transport served it.
//
// no-store rather than the no-cache a redraw uses, and what separates them is
// what each response carries. A redraw carries an entity tag, so no-cache buys
// the conditional request no-store would forbid. A document carries no
// validator at all, so there is no 304 to protect and nothing left to weigh
// against the shared machine, where no-store is what keeps a signed-in page off
// the disk after the browser is closed.
const privateCacheControl = "private, no-store"

// writeChainCachePolicy says whether a shared cache may hold this response.
//
// The answer comes from the chain rather than from the request, exactly as it
// does on the other transport: the templates declare it, so both halves reach
// one verdict from one source rather than from two readings of a rendered
// document.
//
// An undeclared chain is private, which is a framework default rather than a
// property of the annotation. A page treated as shared that is per-reader
// serves one reader's markup to another; a page treated as per-reader that is
// shared costs a cache miss. Those are not comparable.
//
// What is not here is the warning the other half logs when a chain declaring
// public renders private. That report names the component to change, and it is
// about the templates rather than about the transport, so one build of an
// application saying it is enough.
func writeChainCachePolicy(r *fasthttp.RequestCtx, wrappers []HTMLWrapper, leaf HTMLFragment) {
	if htmlbind.IsPrivate(wrappers, leaf) {
		r.Response.Header.Set("Cache-Control", privateCacheControl)
	}
}

// varyOnDeclaredAxes names the request properties a render depends on, so a
// shared cache keyed on the URL alone does not serve one reader's answer to
// another.
func varyOnDeclaredAxes(r *fasthttp.RequestCtx, axes []string) {
	for _, axis := range axes {
		addVaryHeader(r, axis)
	}
}

// addVaryHeader adds one axis unless it is already covered. A Vary of * subsumes
// every axis there could be, so nothing is added beside it.
func addVaryHeader(r *fasthttp.RequestCtx, value string) {
	present := false
	r.Response.Header.VisitAll(func(name, line []byte) {
		if present || !strings.EqualFold(string(name), "Vary") {
			return
		}
		for _, existing := range strings.Split(string(line), ",") {
			existing = strings.TrimSpace(existing)
			if existing == "*" || strings.EqualFold(existing, value) {
				present = true
				return
			}
		}
	})
	if !present {
		r.Response.Header.Add("Vary", value)
	}
}
