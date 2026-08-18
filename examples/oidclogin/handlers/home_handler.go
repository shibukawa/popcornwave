package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornweb/plugin/auth"
	"github.com/shibukawa/popcornweb/pw"
)

func init() { mux.HandleFunc("GET /{$}", home) }

// home is public. It renders either the login entry point or the signed-in
// summary, so the sample shows both states without a second route.
func home(w http.ResponseWriter, r *http.Request) {
	user, authenticated := auth.User(r.Context())
	pw.WriteHTML(w, r, Home(HomeParams{
		Authenticated: authenticated,
		DisplayName:   user.DisplayName,
		Email:         user.Email,
	}))
}
