package pwfast

import (
	"github.com/shibukawa/tinybind-go/fasthttpbind"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// The two calls a generated page dispatcher makes, in the shape this transport
// passes them: one value carrying the request and the response together, so the
// writer and the request are not separate arguments to drop.

// ActionSelectorField is the hidden field a lowered form carries to say which
// server function a native submit is for. It matches the net/http half, so one
// generated form works on either backend.
const ActionSelectorField = fasthttpbind.DefaultActionSelectorField

// ActionSelector returns the server function selector a native form submit
// carried, or the empty string when it carried none.
//
// The query is read before the body, because a submit button's formaction is
// what carries the selector when one form dispatches to several handlers, and
// that channel has to win over the form's own hidden field.
func ActionSelector(r *fasthttp.RequestCtx, field string) string {
	return fasthttpbind.ActionSelector(r, field)
}

// DispatchAction runs one server function on the page's own POST route and
// applies the post-redirect-get default.
//
// A handler that writes nothing is answered with a redirect back to the page it
// was submitted from; one that wrote a status, a header, or a body keeps exactly
// that response.
//
// The redirect goes through Redirect rather than the module's own, for the
// reason the net/http half gives: the runtime posts an intercepted form to this
// same route, so a silent handler's redirect is commonly answered to a fetch,
// which would follow a 303 and apply a whole page where a region set belongs.
//
// The observation compares the response before and after rather than wrapping a
// writer, because this transport carries the request and the response in one
// value and there is nothing to wrap.
func DispatchAction(r *fasthttp.RequestCtx, handler func(*fasthttp.RequestCtx)) {
	if r == nil {
		return
	}
	headers := r.Response.Header.Len()
	handler(r)
	// fasthttp starts a response at 200, so a status still reading 200 is one
	// the handler did not set rather than one it chose. A body stream counts as
	// a body without being read: Response.Body would drain the stream to
	// answer, which for a live subscription means consuming it entirely.
	if r.Response.StatusCode() != fasthttp.StatusOK ||
		r.Response.IsBodyStream() ||
		len(r.Response.Body()) > 0 ||
		r.Response.Header.Len() != headers {
		return
	}
	RedirectSeeOther(r, string(r.RequestURI()))
}

// ActionCallHeader says a server function was called by name from a script
// rather than reached by a gesture, matching the net/http half so one document
// drives either backend.
const ActionCallHeader = "Pw-Call"

// WantsValue reports a server function called from a script, which is the third
// branch of an action handler beside WantsUpdate and the ordinary response.
func WantsValue(r *fasthttp.RequestCtx) bool {
	return r != nil && len(r.Request.Header.Peek(ActionCallHeader)) > 0
}
