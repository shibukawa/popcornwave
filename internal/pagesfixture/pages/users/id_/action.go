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
func Rename(w http.ResponseWriter, r *http.Request) {
	request, err := pw.Parse[renameRequest](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	pw.WriteAPI(w, r, renameResponse{Name: request.Name})
}

// Retire is the form half of the same surface: a handler a form names rather
// than a button, so generation registers POST on the page's own pattern beside
// its GET and the lowered form reaches it with no browser runtime.
//
// It writes nothing, which is what makes the generated dispatcher answer 303
// back to the page. That is the post-redirect-get default, and a handler that
// wrote a status, a header, or a body would keep exactly that response instead.
func Retire(w http.ResponseWriter, r *http.Request) {}
