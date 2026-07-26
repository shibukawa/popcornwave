package handlers

import (
	"net/http"
	"net/url"

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
	app := pw.Config[AppConfig](r.Context())
	// The framework resolved the session before this handler ran; the login
	// itself lives at the configured auth paths, not here.
	identity, signedIn := pw.CurrentUser(r.Context())
	auth := pw.Config[pw.AuthConfig](r.Context())
	pw.WriteHTML(w, r, Home(HomeParams{
		Count:         counter.Count,
		EnvLabel:      app.EnvLabel,
		EnvLabelColor: app.EnvLabelColor,
		AuthEnabled:   auth.Enabled,
		SignedIn:      signedIn,
		UserName:      displayName(identity),
		LoginPath:     url.URL{Path: auth.LoginPath},
		LogoutPath:    url.URL{Path: auth.LogoutPath},
	}))
}

// displayName prefers the provider's name claim and falls back to the subject,
// which every provider guarantees.
func displayName(identity pw.Identity) string {
	if identity.Name != "" {
		return identity.Name
	}
	return identity.Subject
}
