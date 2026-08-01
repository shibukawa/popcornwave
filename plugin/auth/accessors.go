package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/shibukawa/popcornwave/session"
)

// Session returns the validated login session of the request.
func Session(ctx context.Context) (session.View[SessionData], bool) {
	return session.Read[SessionData](ctx)
}

// EstablishSession creates the login session of an account this application
// authenticated through a flow the framework does not own, and writes its
// cookie to w. It rotates whatever session the browser already held, exactly as
// the built-in login endpoints do.
//
// It authenticates nobody: the caller has already decided that this request
// belongs to this account, and is responsible for having verified something
// before deciding it. Nothing a remote client sends can reach this function.
//
// method labels the session for data:request-authentication and appears as
// pw.RequestAuthentication(ctx).Method. Use MethodOIDC, MethodPasskey, or an
// application-defined name.
func EstablishSession(w http.ResponseWriter, r *http.Request, data SessionData, method string) error {
	rt := activeRuntime()
	if rt == nil {
		return errors.New("auth: no authentication runtime; is auth.enabled set?")
	}
	if data.AccountID == "" {
		return errors.New("auth: a session needs an account identifier")
	}
	if method == "" {
		method = MethodOIDC
	}
	return rt.manager.RotateWithMethod(w, r, data, method)
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
