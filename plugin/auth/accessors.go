package auth

import (
	"context"

	"github.com/shibukawa/popcornwave/session"
)

// Session returns the validated login session of the request.
func Session(ctx context.Context) (session.View[SessionData], bool) {
	return session.Read[SessionData](ctx)
}

// User returns the stored account summary of the request. Handlers use it to
// render an authenticated page; authorization decisions must still consult
// application policy.
func User(ctx context.Context) (SessionData, bool) {
	view, ok := Session(ctx)
	if !ok {
		return SessionData{}, false
	}
	return view.Data, true
}
