package pw

import (
	"net/http"

	tinybind "github.com/shibukawa/tinybind-go"
)

// The two calls a generated page dispatcher makes.
//
// A page whose template declares a form action registers POST on its own path
// beside its GET, and the generated body reads which server function was
// submitted and runs it. Both halves are named on the framework namespace, so
// generated code calls pw rather than reaching past it into the module — the
// same containment every other generated call already has.

// ActionSelectorField is the hidden field a lowered form carries to say which
// server function a native submit is for.
//
// It is fixed rather than configured. The template compiler writes the field and
// the generated dispatcher reads it, and both take their name from the same
// place, so a project renaming one without the other would produce a form that
// submits to a dispatcher that cannot tell what it received.
const ActionSelectorField = tinybind.DefaultActionSelectorField

// ActionSelector returns the server function selector a native form submit
// carried, or the empty string when it carried none.
//
// The query is read before the body, because a submit button's formaction is
// what carries the selector when one form dispatches to several handlers, and
// that channel has to win over the form's own hidden field rather than merely
// coexist with it.
func ActionSelector(r *http.Request, field string) string {
	return tinybind.ActionSelector(r, field)
}

// DispatchAction runs one server function on the page's own POST route and
// applies the post-redirect-get default.
//
// A handler that writes nothing is answered with a redirect back to the page it
// was submitted from, so a reload does not resubmit and the address bar keeps
// showing the page. A handler that wrote a status, a header, or a body keeps
// exactly that response, which is what lets it redirect elsewhere, render the
// page inline with validation errors, or stream.
//
// The redirect goes through Redirect rather than the module's own, which is the
// one thing this wrapper adds. The runtime posts an intercepted form to this
// same route, so the redirect a silent handler produces is commonly answered to
// a fetch — and a fetch follows a 303 and would apply a whole page where a
// region set belongs. Redirect already branches on that, so routing through it
// means a scriptless submit and an intercepted one reach the same handler and
// each gets the answer its own client can use.
func DispatchAction(w http.ResponseWriter, r *http.Request, handler http.HandlerFunc) {
	observer := newActionResponse(w)
	handler(observer, r)
	if observer.wrote() {
		return
	}
	RedirectSeeOther(w, r, r.URL.RequestURI())
}

// actionResponse observes whether a handler produced a response of its own, so
// a handler needs no flag and no framework type to choose between answering and
// letting the default redirect stand.
//
// It counts the headers present before the handler ran, so a header a
// middleware installed is not mistaken for one the handler wrote.
type actionResponse struct {
	http.ResponseWriter
	headers int
	status  bool
	body    bool
}

func newActionResponse(w http.ResponseWriter) *actionResponse {
	return &actionResponse{ResponseWriter: w, headers: len(w.Header())}
}

func (a *actionResponse) WriteHeader(status int) {
	a.status = true
	a.ResponseWriter.WriteHeader(status)
}

func (a *actionResponse) Write(p []byte) (int, error) {
	a.body = true
	return a.ResponseWriter.Write(p)
}

// wrote reports whether the handler answered for itself.
func (a *actionResponse) wrote() bool {
	return a.status || a.body || len(a.Header()) > a.headers
}
