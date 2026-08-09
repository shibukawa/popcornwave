package pwfast

import (
	"github.com/shibukawa/tinybind-go/htmlbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// htmlContentType is what a rendered document and a rendered fragment both
// carry. It is spelled here rather than read from a negotiation, because these
// entries render HTML by definition; what varies between them is the shell,
// not the media type.
const htmlContentType = "text/html; charset=utf-8"

// WriteHTMLChain renders generated wrappers around one leaf.
//
// Nothing commits until the whole chain has rendered, so a failure part way
// through still becomes a problem response rather than a truncated document.
// A chain carrying await boundaries settles them before writing: this half
// renders buffered where the net/http one can stream, which costs time to first
// byte and changes no bytes. Streaming is what the deferred htmlupdate port
// unlocks, since the boundary delivery path holds a flusher.
func WriteHTMLChain(r *fasthttp.RequestCtx, wrappers []HTMLWrapper, leaf HTMLFragment, options ...HTMLOption) {
	var body []byte
	buffer := bytesWriter{buf: &body}
	if err := htmlbind.RenderChain(&buffer, wrappers, leaf, options...); err != nil {
		WriteProblem(r, err)
		return
	}
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
