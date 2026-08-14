package id_

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

type renameRequest struct {
	Name string `json:"name" check:"required"`
}

type renameResponse struct {
	Name string `json:"name"`
}

// Rename is a server action: an exported handler in a route package, reachable
// at a generated address, owning its whole response.
//
// It reads a typed request, which only works because generation runs over the
// packages of a page tree. The binder it calls is generated from this file.
//
// It answers by caller, which is what owning the whole response is for. A script
// called this and is holding the answer, so it gets a value; anyone else gets
// the page again, because a gesture has a document to update and nowhere to put
// one. A handler with nothing to return asks neither question and writes one
// response for everybody.
func Rename(w http.ResponseWriter, r *http.Request) {
	request, err := pw.Parse[renameRequest](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if pw.WantsValue(r) {
		pw.WriteAPI(w, r, renameResponse{Name: request.Name})
		return
	}
	// A fixed destination rather than the page this came from, because it cannot
	// be reconstructed here: a bare button posts to the direct endpoint, whose
	// address is a constant carrying none of the route's path parameters. A
	// handler that needs to send somebody back to their own page belongs on a
	// form, which posts to the page URL.
	pw.RedirectSeeOther(w, r, "/")
}

// Retire is the form half of the same surface: a handler a form names rather
// than a button, so generation registers POST on the page's own pattern beside
// its GET and the lowered form reaches it with no browser runtime.
//
// It writes nothing, which is what makes the generated dispatcher answer 303
// back to the page. That is the post-redirect-get default, and a handler that
// wrote a status, a header, or a body would keep exactly that response instead.
func Retire(w http.ResponseWriter, r *http.Request) {}

// A typed server action belongs here, and one is not declared yet: v0.5.10
// emits its wrapper into every per-source artifact of the package while
// planning its argument struct under an empty source path, so the wrapper and
// the decoder it names land in different files. docs/tinybind-go-typed-action-
// wiring-report.md carries what was measured; the wiring on this side is done
// and this fixture is where the declaration goes when that is answered.
