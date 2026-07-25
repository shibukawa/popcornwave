package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
	"helloworld/queries"
)

func init() { mux.HandleFunc("GET /", home) }

func home(w http.ResponseWriter, r *http.Request) {
	counter, err := queries.IncrementAccess(r.Context())
	if err != nil {
		pw.WriteProblem(w, r, pw.InternalServerError(err))
		return
	}
	pw.WriteHTML(w, r, Home(HomeParams{Count: counter.Count}))
}
