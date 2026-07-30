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
