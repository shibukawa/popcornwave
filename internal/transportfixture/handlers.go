// Package transportfixture is authored handler code written the way an
// application writes it, so the transport analysis has something in this
// repository to run against.
//
// It exists because nothing else here is a valid input. Generated files are
// emitted per backend rather than rewritten, and the examples are each their own
// module whose query and template packages exist only after pw generate. This
// package compiles with the rest of the tree, so a pw entry that loses its call
// pattern is caught by the build and by the analysis together.
//
// Every handler below is deliberately ordinary. The point is not to exercise
// unusual shapes but to hold the calls an application actually makes, so that
// the set of registered patterns is checked against the set that gets used.
package transportfixture

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/tinybind-go/htmlbind"
)

// Greeting is the response type an API handler writes.
type Greeting struct {
	Message string `json:"message"`
}

// APIHandler binds a request and answers with a typed value, which is the
// shortest path through the framework an application takes.
func APIHandler(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[struct{}](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	_ = input
	pw.WriteAPI(w, r, Greeting{Message: "hello"})
}

// PageHandler renders a page inside the registered document shell and reports a
// problem the same way every other handler does.
func PageHandler(w http.ResponseWriter, r *http.Request) {
	if pw.IsBot(r) {
		pw.WriteHTMLFragment(w, r, fragment("<p>crawler</p>"))
		return
	}
	pw.WriteHTML(w, r, fragment("<main>page</main>"))
}

// ActionHandler is the shape requirement:action-response-update describes: one
// branch, the regions on one side and the ordinary response on the other.
func ActionHandler(w http.ResponseWriter, r *http.Request) {
	if pw.WantsUpdate(r) {
		pw.WriteUpdate(w, r, http.StatusOK, pw.Replace("total", fragment(`<b id="total">1</b>`)))
		return
	}
	pw.WriteUpdateNavigate(w, r, "/orders")
}

// StreamHandler writes a typed event stream through the callback entry.
func StreamHandler(w http.ResponseWriter, r *http.Request) {
	pw.WriteStream(w, r, func(stream *pw.Stream[Greeting]) error {
		return stream.Write(Greeting{Message: "tick"})
	})
}

// SpecHandler serves the generated specification, which is a pw entry an
// application mounts rather than writes.
func SpecHandler(w http.ResponseWriter, r *http.Request) {
	pw.OpenAPIJSON(w, r)
}

// renderError is the shared helper the eligibility rule exists to carry: it
// takes the transport and is called from a handler, so a transform that refused
// it would refuse everything that calls it.
func renderError(w http.ResponseWriter, r *http.Request, err error) {
	pw.WriteProblem(w, r, pw.InternalServerError(err))
}

// HelperCallingHandler is why the admission set closes over the call graph.
func HelperCallingHandler(w http.ResponseWriter, r *http.Request) {
	renderError(w, r, http.ErrNoCookie)
}

func fragment(markup string) pw.HTMLFragment {
	builder := htmlbind.Builder[struct{}]{}
	return htmlbind.Bind(&htmlbind.Plan[struct{}]{
		Ops: []htmlbind.Op[struct{}]{builder.Static(markup)},
	}, struct{}{})
}
