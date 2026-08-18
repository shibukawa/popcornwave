//go:build !fasthttp

package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornweb/pw"
	"helloworld/queries"
)

func init() { mux.HandleFunc("GET /{$}", home) }

func home(w http.ResponseWriter, r *http.Request) {
	counter, err := queries.IncrementAccess(r.Context())
	if err != nil {
		pw.WriteProblem(w, r, pw.InternalServerError(err))
		return
	}
	app := pw.Config[AppConfig](r)
	pw.WriteHTML(w, r, Home(HomeParams{
		Count:         counter.Count,
		EnvLabel:      app.EnvLabel,
		EnvLabelColor: app.EnvLabelColor,
	}))
}
