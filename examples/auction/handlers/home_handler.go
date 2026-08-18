package handlers

import (
	"net/http"
	"net/url"

	"github.com/shibukawa/popcornweb/plugin/auth"
	"github.com/shibukawa/popcornweb/pw"
)

func init() { mux.HandleFunc("GET /hello/{$}", home) }

// The comment below is not decoration. pw generate reads a handler's godoc into
// the OpenAPI document this project serves: the first sentence becomes the
// operation summary and the rest its description. Write them and /docs explains
// the route; leave them out and it lists a path and nothing else.
//
// This paragraph is separated by a blank line, so it is not part of that godoc
// and does not reach the document.

// home renders the starter landing page, signed in or not.
//
// A signed-in visitor is greeted by display name and offered a sign-out
// control; everyone else is offered the ways in that this project configured.
func home(w http.ResponseWriter, r *http.Request) {
	// The framework resolved the session before this handler ran.
	user, signedIn := auth.User(r.Context())
	name := "World"
	if signedIn {
		name = user.DisplayName
		if name == "" {
			name = user.Subject
		}
	}
	pw.WriteHTML(w, r, Home(HomeParams{
		Name:          name,
		Project:       "auction",
		SignedIn:      signedIn,
		Email:         user.Email,
		LoginPath:     url.URL{Path: "/auth/login"},
		LogoutPath:    url.URL{Path: "/auth/logout"},
		Passkey:       false,
		ProviderLogin: true,
		Bootstrap:     false,
	}))
}
