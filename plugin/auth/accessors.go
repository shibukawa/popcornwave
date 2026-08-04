package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/session"
)

// sessionSlotKey names this package's slot inside a session record.
const sessionSlotKey = "pw_auth"

// registerSessionSlot declares the login half of a session, from this package's
// init, exactly as an application declares its own.
//
// It is session.Private, which is the default placement: sealed either way, on
// a cookie while the session is anonymous and on the configured backend from
// the login onward. Nothing here is privileged in storage.
func registerSessionSlot() {
	pw.RegisterSessionStore[SessionData](sessionSlotKey, session.Private)
}

// Session returns the validated login session of the request.
func Session(ctx context.Context) (SessionData, bool) {
	data, ok := session.Load[SessionData](ctx)
	if !ok || data.AccountID == "" {
		return SessionData{}, false
	}
	return data, true
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
	return rt.establish(w, r, data, method)
}

// establish writes the login slot and rotates the session.
//
// Rotation is the fixation defense, and it is also the promotion: a slot that
// rode a sealed cookie while the browser was anonymous lands on the configured
// backend here, so anything the visitor accumulated before signing in survives
// the sign-in.
func (rt *runtime) establish(w http.ResponseWriter, r *http.Request, data SessionData, method string) error {
	if method == "" {
		method = MethodOIDC
	}
	data.Method = method
	if data.AuthenticatedAt.IsZero() {
		data.AuthenticatedAt = time.Now().UTC()
	}
	// A login normally arrives through the session middleware. Attach covers
	// the callers that legitimately have nothing above them, such as the test
	// seam, and is a no-op when the middleware already ran.
	r, err := rt.manager.Attach(w, r)
	if err != nil {
		return err
	}
	handle, ok := session.Value[SessionData](r.Context())
	if !ok {
		return errors.New("auth: the session package has no slot for the login")
	}
	if err := handle.Set(data); err != nil {
		return err
	}
	return rt.manager.Rotate(w, r)
}

// endSession destroys the whole session: every record is revoked and every
// cookie it owns is expired, whatever placement each slot carries.
func (rt *runtime) endSession(w http.ResponseWriter, r *http.Request) error {
	return rt.manager.Destroy(w, r)
}

// User returns the stored account summary of the request. Handlers use it to
// render an authenticated page; authorization decisions must still consult
// application policy.
func User(ctx context.Context) (SessionData, bool) {
	return Session(ctx)
}
