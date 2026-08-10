package auth

import (
	"context"
	"errors"
	"github.com/shibukawa/popcornwave/pwsession"
	"net/http"
	"time"

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
//
// Private rather than session.ServerOnly, even though a login wants to be
// revocable, because this init cannot see whether auth is enabled and
// ServerOnly would refuse the cookie backend outright for a deployment that
// merely links this package. setupAuthentication warns about that pairing
// itself, where both the configuration and the environment are known, and dev
// is left alone. Nothing is written here before a login in any case: establish
// is the only writer, so the anonymous phase stores nothing.
func registerSessionSlot() {
	pwsession.RegisterStore[SessionData](sessionSlotKey, session.Private)
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
	return EstablishSessionOn(HTTPExchange(w, r), data, method)
}

// EstablishSessionOn is EstablishSession over the transport seam, for an
// application serving on a transport whose request is not a *http.Request.
func EstablishSessionOn(x Exchange, data SessionData, method string) error {
	rt := activeRuntime()
	if rt == nil {
		return errors.New("auth: no authentication runtime; is auth.enabled set?")
	}
	if data.AccountID == "" {
		return errors.New("auth: a session needs an account identifier")
	}
	return rt.establish(x, data, method)
}

// establish writes the login slot and rotates the session.
//
// Rotation is the fixation defense, and it is also the promotion: a slot that
// rode a sealed cookie while the browser was anonymous lands on the configured
// backend here, so anything the visitor accumulated before signing in survives
// the sign-in.
func (rt *runtime) establish(x Exchange, data SessionData, method string) error {
	if method == "" {
		method = MethodOIDC
	}
	data.Method = method
	if data.AuthenticatedAt.IsZero() {
		data.AuthenticatedAt = time.Now().UTC()
	}
	// A login normally arrives through the session middleware. Attaching covers
	// the callers that legitimately have nothing above them, such as the test
	// seam, and is a no-op when the middleware already ran.
	if err := rt.attachSession(x); err != nil {
		return err
	}
	handle, ok := session.Value[SessionData](x.Context())
	if !ok {
		return errors.New("auth: the session package has no slot for the login")
	}
	if err := handle.Set(data); err != nil {
		return err
	}
	return rt.manager.RotateOn(x.Context())
}

// endSession destroys the whole session: every record is revoked and every
// cookie it owns is expired, whatever placement each slot carries.
func (rt *runtime) endSession(x Exchange) error {
	return rt.manager.DestroyOn(x.Context())
}

// User returns the stored account summary of the request. Handlers use it to
// render an authenticated page; authorization decisions must still consult
// application policy.
func User(ctx context.Context) (SessionData, bool) {
	return Session(ctx)
}
