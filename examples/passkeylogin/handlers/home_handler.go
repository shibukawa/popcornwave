package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/pw"
)

func init() { mux.HandleFunc("GET /{$}", home) }

// home is public. It renders either the login entry point or the signed-in
// summary, so the sample shows both states without a second route.
func home(w http.ResponseWriter, r *http.Request) {
	user, authenticated := auth.User(r.Context())
	// Which method signed this request in. A passkey session and an OIDC
	// session are the same session once created; only this differs.
	viaPasskey := pw.RequestAuthentication(r.Context()).Method == auth.MethodPasskey
	pw.WriteHTML(w, r, Home(HomeParams{
		Authenticated: authenticated,
		DisplayName:   user.DisplayName,
		Email:         user.Email,
		HasPasskey:    viaPasskey,
	}))
}
