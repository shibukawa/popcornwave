//go:build !fasthttp

package fastfixture

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

// Greet is the authored handler, and the only one written here. Its fasthttp
// counterpart is generated from it.
func Greet(w http.ResponseWriter, r *http.Request) {
	ask, err := pw.Parse[Greeting](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	pw.WriteAPI(w, r, Greeted{Message: "hello, " + ask.Name})
}
