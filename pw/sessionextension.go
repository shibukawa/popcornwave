package pw

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/middlewares"
	"github.com/shibukawa/popcornwave/pwruntime"
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
	stop    chan struct{}
}

// sessionPruneInterval bounds how often expired records are swept, and
// sessionPruneBatch bounds one sweep so a large backlog cannot hold
// connections.
const (
	sessionPruneInterval = 10 * time.Minute
	sessionPruneBatch    = 256
)

// sweepSessions removes records that expired without being revoked.
//
// It runs here rather than in an authentication plugin because the records are
// the framework's: a deployment with no login still writes them for a
// session.ServerOnly slot, and abandonment rather than logout is how most
// sessions end. The cutoff exists because session.retention bounds every
// record, per decision:storage-bounded-session-record.
func sweepSessions(prune func(context.Context, time.Time, int) (int64, error), stop <-chan struct{}) {
	ticker := time.NewTicker(sessionPruneInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, _ = prune(ctx, time.Now(), sessionPruneBatch)
			cancel()
		}
	}
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
	if err := validateDevelopmentSessionMode(config.Backend); err != nil {
		return nil, err
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
	cookieMode := config.Backend == SessionBackendCookie || config.Backend == SessionBackendDevPersist
	if !cookieMode && options.TTL <= 0 {
		// A server record with no deadline is one the sweep has no cutoff for,
		// and the store reads a zero expiry as already past.
		return nil, errors.New(
			"session.retention must be positive for a server backend, because a record with no deadline is never swept")
	}

	var store session.RawStore
	var backend session.Backend
	if !cookieMode {
		resources, err := sessionResources(ctx)
		if err != nil {
			return nil, err
		}
		backend, err = OpenSessionBackend(ctx, config, resources)
		if err != nil {
			return nil, err
		}
		store = backend.Store
	}
	options.ServerSideAnonymous = config.Backend == SessionBackendDevVolatile
	// Cookie and dev-persist are not server stores: the manager seals a record
	// into the browser for an anonymous session already, and either selection
	// means it never moves off there.
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
	// Repeated framework initialization, which tests perform, must not leave an
	// earlier sweep running or an earlier client open.
	if previous := sessionState.stop; previous != nil {
		close(previous)
	}
	if previous := sessionState.close; previous != nil {
		_ = previous(context.Background())
	}
	sessionState.manager, sessionState.close, sessionState.prune = manager, closer, prune
	sessionState.stop = nil
	if prune != nil {
		sessionState.stop = make(chan struct{})
		go sweepSessions(prune, sessionState.stop)
	}
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
		TTL:             recordLifetime(config.Retention, lifetime.TTL),
		IdleTimeout:     lifetime.IdleTimeout,
		RenewalInterval: lifetime.RenewalInterval,
		Cookie:          policy,
		RecordCookie:    session.CookieOptions{Name: config.CookieStore.Name},
		Keys:            keys,
	}, nil
}

// recordLifetime is how long a record actually lives: the shorter of what the
// store will hold and what a proof of identity stays good for.
//
// They bound different things, so neither subsumes the other. A zero on either
// side is that side declining to bound it, which leaves the other one in force;
// zero on both leaves the browser as the only bound, which validateSessionStore
// refuses for a server backend.
func recordLifetime(retention, ttl time.Duration) time.Duration {
	switch {
	case retention <= 0:
		return ttl
	case ttl <= 0:
		return retention
	case ttl < retention:
		return ttl
	default:
		return retention
	}
}

// writeSessionUnavailable fails closed. "The store is unreachable" and "you are
// not signed in" must not look the same to an application deciding what to
// show, so the request is refused rather than downgraded to anonymous.
func writeSessionUnavailable(w http.ResponseWriter, r *http.Request, _ error) {
	WriteProblem(w, r, ServiceUnavailable())
}

// sessionResources opens what a session backend might want.
//
// A project with no relational middleware gets an empty set rather than an
// error: whether a database is needed is the selected backend's question, and
// the rdb backend is the one that answers it. Resolving eagerly here would
// refuse a DynamoDB or Redis session for the absence of something it never
// reads.
func sessionResources(ctx context.Context) (SessionResources, error) {
	if _, enabled := pwruntime.ConnectionExecutor(ctx); !enabled {
		return SessionResources{}, nil
	}
	// The session record is written on every change, so it lives in the session
	// group rather than in the default group, which is normally a replica.
	sessionCtx, err := SelectSessionDB(ctx)
	if err != nil {
		return SessionResources{}, err
	}
	db, _ := DB(sessionCtx)
	executor, _ := pwruntime.ConnectionExecutor(sessionCtx)
	driver, _ := DBDriver(sessionCtx)
	return SessionResources{DB: db, Executor: executor, DBDriver: driver}, nil
}
