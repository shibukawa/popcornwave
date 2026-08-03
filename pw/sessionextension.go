package pw

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/middlewares"
	"github.com/shibukawa/popcornwave/session"
	"github.com/shibukawa/popcornwave/sessionconfig"
)

func init() {
	RegisterExtension(Extension{
		Name:  "session",
		Slot:  SlotSession,
		Setup: setupSession,
		Close: closeSession,
	})
}

// sessionState holds the manager of the current initialization, so that a later
// slot can reach the session an earlier slot resolved. Framework initialization
// runs more than once in one process, most obviously in tests, so it is
// replaced rather than assumed to be written once.
var sessionState struct {
	sync.RWMutex
	manager *session.Manager
	close   func(context.Context) error
	prune   func(context.Context, time.Time, int) (int64, error)
}

// SessionManager returns the manager the session middleware installed, or nil
// when session storage is disabled.
//
// It is how popcornwave/plugin/auth reaches Rotate and Destroy: the framework
// resolves the session at SlotSession and authentication drives it at
// SlotAuthentication, which is why the manager travels forward and the login
// does not travel back.
func SessionManager() *session.Manager {
	sessionState.RLock()
	defer sessionState.RUnlock()
	return sessionState.manager
}

// SessionPrune returns the expiry sweep of the configured backend, or nil for a
// backend whose server or browser forgets records on its own.
func SessionPrune() func(context.Context, time.Time, int) (int64, error) {
	sessionState.RLock()
	defer sessionState.RUnlock()
	return sessionState.prune
}

// setupSession resolves per-browser state for every request.
//
// It knows nothing about login. The durations it enforces are read from the
// [auth.session] binding, which exists only when an authentication plugin is
// linked; without one the session is bounded by the browser alone, which is
// what decision:session-lifetime-owned-by-auth accepts.
func setupSession(ctx context.Context) (Middleware, error) {
	replaceSession(nil, nil, nil)

	config := Config[SessionConfig](ctx)
	if !config.Enabled {
		return nil, nil
	}
	registry, err := newSessionRegistry()
	if err != nil {
		return nil, err
	}
	// The CSRF secret is a slot like any other, declared here rather than from
	// an init so that a project with the check off carries no slot and needs no
	// keyring on its account.
	if Config[SecurityConfig](ctx).CSRF.Enabled {
		if err := session.Register[middlewares.CSRFSecret](
			registry, middlewares.CSRFSecretSlot, session.Private, nil,
			session.ResetOnRotate()); err != nil {
			return nil, err
		}
	}
	options, err := sessionOptions(config, Config[sessionconfig.SessionLifetimeConfig](ctx))
	if err != nil {
		return nil, err
	}

	var store session.RawStore
	var backend session.Backend
	if config.Backend != SessionBackendCookie {
		// The session record is written on every change, so it lives in the
		// session group rather than in the default group, which is normally a
		// replica.
		sessionCtx, err := SelectSessionDB(ctx)
		if err != nil {
			return nil, err
		}
		db, _ := DB(sessionCtx)
		driver, _ := DBDriver(sessionCtx)
		backend, err = OpenSessionBackend(ctx, config, SessionResources{DB: db, DBDriver: driver})
		if err != nil {
			return nil, err
		}
		store = backend.Store
	}
	// The cookie backend is not a server store: the manager seals a record into
	// the browser for an anonymous session already, and selecting cookie means
	// it never moves off there.
	manager, err := session.NewManager(registry, store, options)
	if err != nil {
		return nil, err
	}
	replaceSession(manager, backend.Close, backend.Prune)
	return manager.Middleware(writeSessionUnavailable), nil
}

func replaceSession(manager *session.Manager, closer func(context.Context) error, prune func(context.Context, time.Time, int) (int64, error)) {
	sessionState.Lock()
	defer sessionState.Unlock()
	if previous := sessionState.close; previous != nil {
		_ = previous(context.Background())
	}
	sessionState.manager, sessionState.close, sessionState.prune = manager, closer, prune
}

func closeSession(context.Context) error {
	replaceSession(nil, nil, nil)
	return nil
}

// sessionOptions maps the two bindings onto what the session package enforces.
// The placement and the cookie policy come from [session]; every duration comes
// from [auth.session], because an expiry states how long a proof of identity
// stays good and the store holding the bytes has no basis to make it.
func sessionOptions(config SessionConfig, lifetime sessionconfig.SessionLifetimeConfig) (session.Options, error) {
	policy, err := SessionCookiePolicy(config)
	if err != nil {
		return session.Options{}, err
	}
	// One secret protects everything the browser carries: a signed slot and a
	// sealed one derive purpose-separated subkeys from it.
	keys, err := SessionKeyring(config.Keyring)
	if err != nil {
		return session.Options{}, err
	}
	return session.Options{
		TTL:             lifetime.TTL,
		IdleTimeout:     lifetime.IdleTimeout,
		RenewalInterval: lifetime.RenewalInterval,
		Cookie:          policy,
		RecordCookie:    session.CookieOptions{Name: config.CookieStore.Name},
		Keys:            keys,
	}, nil
}

// writeSessionUnavailable fails closed. "The store is unreachable" and "you are
// not signed in" must not look the same to an application deciding what to
// show, so the request is refused rather than downgraded to anonymous.
func writeSessionUnavailable(w http.ResponseWriter, r *http.Request, _ error) {
	WriteProblem(w, r, ServiceUnavailable())
}
