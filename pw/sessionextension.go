package pw

import (
	"context"
	"net/http"
	"time"

	"github.com/shibukawa/popcornweb/pwsession"
	"github.com/shibukawa/popcornweb/session"
	"github.com/shibukawa/popcornweb/sessionconfig"
)

func init() {
	RegisterExtension(Extension{
		Name:  "session",
		Slot:  SlotSession,
		Setup: setupSession,
		Close: pwsession.Close,
	})
}

// The session layer lives in popcornweb/pwsession, and every entry below is a
// thin wrapper or a true alias of one declared there.
//
// It moved because almost none of a session is transport-shaped: the slot
// declarations, the backend registry, the keyring, the cookie policy, the
// lifetime arithmetic and the expiry sweep read the same whichever server
// answered. What this runtime keeps is the one frame that hands the resolved
// manager a net/http request, and the answer it gives when the store is down.
type (
	// SessionResources are the framework resources a session backend may
	// borrow. A backend closes nothing it finds here: what it did not open, it
	// does not own.
	SessionResources = pwsession.Resources
	// SessionBackendFactory opens one storage backend from configuration.
	SessionBackendFactory = pwsession.BackendFactory
)

// RegisterSessionBackend registers factory under name. A storage plugin calls
// it from init, so a blank import is what puts a backend in a binary:
//
//	import _ "github.com/shibukawa/popcornweb/sessionstore/redis"
func RegisterSessionBackend(name string, factory SessionBackendFactory) {
	pwsession.RegisterBackend(name, factory)
}

// SessionBackends lists the registered backend names in order. It is what the
// startup summary and error messages report.
func SessionBackends() []string { return pwsession.Backends() }

// OpenSessionBackend opens the backend named by config.Backend.
func OpenSessionBackend(ctx context.Context, config SessionConfig, resources SessionResources) (session.Backend, error) {
	return pwsession.OpenBackend(ctx, config, resources)
}

// SessionCookiePolicy resolves the validated browser cookie policy of the
// session middleware. The cookie backend and the session manager share it, so
// both halves of a session travel under one policy.
func SessionCookiePolicy(config SessionConfig) (session.CookieOptions, error) {
	return pwsession.CookiePolicy(config)
}

// SessionKeyring reads the secret that protects everything the browser carries.
//
// One secret serves both protections a slot can carry: session.ReadOnly signs
// and session.Private seals, and session.Keyring derives a purpose-separated
// subkey per mode. It is therefore required unless every declared slot is
// session.Shared, which protects nothing, or session.RequestScope, which never
// leaves process memory.
//
// The secret itself never reaches an error message or a log.
func SessionKeyring(config SessionKeyringConfig) (*session.Keyring, error) {
	return pwsession.Keyring(config)
}

// RegisterSessionStore declares one piece of per-browser state, as a Go type
// with a placement, and is the only place either is stated.
//
//	pw.RegisterSessionStore[Cart]("cart", session.Private)
//	cart, ok := session.Load[Cart](ctx)
//
// The placement states what the client may do with the value and where its
// bytes live: session.Shared is a plain cookie the front end reads and writes,
// session.ReadOnly a signed one it may read, session.Private is sealed and
// moves from a cookie to the configured backend at the login rotation,
// session.ServerOnly is sealed and always on the server because it must stay
// revocable, and session.RequestScope lives in process memory for one request
// and is never persisted. The deployment is left with one choice, which server
// backend session.backend names.
//
// How long the value lives is the second, independent question. Stating nothing
// ties it to the session. session.ExpiresAfter ends it earlier, which is what a
// rotating secret or a short admission window wants. session.OutlivesSession
// exempts it from a sign-out, which is what a display language wants, and is
// available only to the two cookie-placed values because a record cannot
// outlive its own destruction.
//
// Call it from main, after every package init has run, exactly as
// RegisterConfig requires: the declarations must be complete before the first
// request decodes anything.
func RegisterSessionStore[T any](key string, placement session.Placement, options ...session.SlotOption) {
	pwsession.RegisterStore[T](key, placement, options...)
}

// SessionRegistry builds the registry of everything declared through
// RegisterSessionStore, plus the slots the caller adds.
//
// The framework half of a login is one such slot, so plugin/auth passes its own
// declaration here rather than being privileged in storage.
func SessionRegistry(extra ...func(*session.Registry) error) (*session.Registry, error) {
	return pwsession.NewRegistry(extra...)
}

// SessionManager returns the manager the session middleware installed, or nil
// when session storage is disabled.
func SessionManager() *session.Manager { return pwsession.Manager() }

// SessionPrune returns the expiry sweep of the configured backend, or nil for a
// backend whose server or browser forgets records on its own.
func SessionPrune() func(context.Context, time.Time, int) (int64, error) { return pwsession.Prune() }

// sessionOptions maps the two bindings onto what the session package enforces.
func sessionOptions(config SessionConfig, lifetime sessionconfig.SessionLifetimeConfig) (session.Options, error) {
	return pwsession.Options(config, lifetime)
}

// setupSession resolves per-browser state for every request.
//
// The resolution is pwsession's and the frame is this transport's, which is the
// whole of what differs: the other half installs pwfast.Session over the same
// manager, so one browser is governed by one set of rules however it arrived.
func setupSession(ctx context.Context) (Middleware, error) {
	manager, err := pwsession.Setup(ctx)
	if err != nil || manager == nil {
		return nil, err
	}
	return manager.Middleware(writeSessionUnavailable), nil
}

func closeSession(ctx context.Context) error { return pwsession.Close(ctx) }

// writeSessionUnavailable fails closed. "The store is unreachable" and "you are
// not signed in" must not look the same to an application deciding what to
// show, so the request is refused rather than downgraded to anonymous.
func writeSessionUnavailable(w http.ResponseWriter, r *http.Request, _ error) {
	WriteProblem(w, r, ServiceUnavailable())
}
