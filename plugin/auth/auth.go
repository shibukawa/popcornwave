// Package auth adds session-backed browser authentication to a Popcorn Wave
// application. Importing it registers the [auth] configuration binding and the
// framework extensions that resolve sessions, own the login endpoints, and
// guard protected paths.
//
//	import _ "github.com/shibukawa/popcornwave/plugin/auth"
//
// The current build implements auth.mode = "oidc_only": OpenID Connect
// Authorization Code with PKCE against one configured issuer, a login session
// in whatever backend session.backend selects, and single-use OAuth
// correlation state stored by contrib/authstate/sqlite.
//
// This package imports no storage plugin. It asks pw for the configured
// backend, so an application links the storage it configured and no more, and
// a backend other than cookie needs its own blank import.
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	authsqlite "github.com/shibukawa/popcornwave/contrib/authstate/sqlite"
	"github.com/shibukawa/popcornwave/contrib/oauth"
	"github.com/shibukawa/popcornwave/contrib/oidc"
	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/session"
)

// methodOIDC labels sessions created by the OIDC flow.
const methodOIDC = "oidc"

// stateNamespace isolates this package's correlation records in the shared
// auth state table.
const stateNamespace = "auth-oidc"

func init() {
	pw.RegisterExtension(pw.Extension{
		Name:  "auth.session",
		Slot:  pw.SlotSession,
		Setup: setupSession,
		Close: closeRuntime,
	})
	pw.RegisterExtension(pw.Extension{
		Name:  "auth.endpoints",
		Slot:  pw.SlotAuthentication,
		Setup: setupEndpoints,
	})
	pw.RegisterExtension(pw.Extension{
		Name:  "auth.guard",
		Slot:  pw.SlotGuard,
		Setup: setupGuard,
	})
}

// runtime is the state shared by the three extensions of this package. It is
// rebuilt whenever framework initialization runs.
type runtime struct {
	config  Config
	manager *session.Manager[SessionData]
	// sessionPrune is the expiry sweep of the session backend. A backend whose
	// server or browser forgets records on its own leaves it nil.
	sessionPrune func(context.Context, time.Time, int) (int64, error)
	// sessionClose releases a client the backend opened. A backend that
	// borrowed the middleware database leaves it nil.
	sessionClose func(context.Context) error
	stateStore   *authsqlite.Store[oauth.Transaction]
	allowlist    Allowlist
	cookiePolicy pw.SessionCookieConfig
	include      []pattern
	exclude      []pattern
	// stopPruning ends the background expiry sweep during shutdown.
	stopPruning chan struct{}

	// discovery is deferred to the first login so that application startup
	// does not depend on the identity provider being reachable.
	discovery sync.Mutex
	client    *oidc.Client
}

var current struct {
	sync.RWMutex
	runtime *runtime
}

func activeRuntime() *runtime {
	current.RLock()
	defer current.RUnlock()
	return current.runtime
}

// setupSession validates configuration, opens session storage, and returns the
// session middleware. It runs first, so the later slots only read the prepared
// runtime.
func setupSession(ctx context.Context) (pw.Middleware, error) {
	replaceRuntime(nil)

	config := pw.Config[Config](ctx)
	if !config.Enabled {
		return nil, nil
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	sessionConfig := pw.Config[pw.SessionConfig](ctx)
	if !sessionConfig.Enabled {
		return nil, errors.New("auth requires session.enabled = true")
	}
	// Whatever the session backend is, this package needs the database: the
	// single-use OAuth correlation records and the admission allowlist are
	// server state in every backend.
	if _, ok := pw.DB(ctx); !ok {
		return nil, errors.New("auth requires middleware.rdb.enabled = true")
	}
	// The session table is written on every login, so it lives in the session
	// group rather than in the default group, which is normally a replica.
	sessionCtx, err := pw.SelectSessionDB(ctx)
	if err != nil {
		return nil, err
	}
	db, ok := pw.DB(sessionCtx)
	if !ok {
		return nil, errors.New("auth requires middleware.rdb.enabled = true")
	}
	if driver, _ := pw.DBDriver(sessionCtx); driver != "sqlite" {
		// The OAuth correlation store is the SQLite implementation of
		// contrib/authstate; another dialect needs its own adapter.
		return nil, fmt.Errorf("auth currently requires a sqlite middleware.rdb.dsn, got driver %q", driver)
	}

	options, err := sessionOptions(sessionConfig)
	if err != nil {
		return nil, err
	}
	// The backend is resolved by name, so this package links no storage
	// plugin and each backend validates its own configuration and schema.
	driver, _ := pw.DBDriver(sessionCtx)
	backend, err := pw.OpenSessionBackend(ctx, sessionConfig, pw.SessionResources{DB: db, DBDriver: driver})
	if err != nil {
		return nil, err
	}
	stateStore, err := authsqlite.NewStore[oauth.Transaction](db, oauth.TransactionCodec{}, authsqlite.Options{
		Namespace: stateNamespace,
	})
	if err != nil {
		return nil, err
	}
	// The tables this package owns are migration-owned, so startup verifies
	// them instead of creating them. The session backend verifies its own.
	schemaCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := verifyTables(schemaCtx, db); err != nil {
		return nil, err
	}
	// The table already exists, so this validates its column layout only.
	if err := stateStore.EnsureSchema(schemaCtx); err != nil {
		return nil, fmt.Errorf("auth state schema: %w", err)
	}
	manager, err := session.NewManager[SessionData](
		session.Typed[SessionData](backend.Store, session.JSONCodec[SessionData]{}), options)
	if err != nil {
		return nil, err
	}
	include, err := compilePatterns(config.Protection.Include)
	if err != nil {
		return nil, err
	}
	exclude, err := compilePatterns(config.Protection.Exclude)
	if err != nil {
		return nil, err
	}
	instance := &runtime{
		config:       config,
		manager:      manager,
		sessionPrune: backend.Prune,
		sessionClose: backend.Close,
		stateStore:   stateStore,
		allowlist:    Allowlist{db: db},
		cookiePolicy: sessionConfig.Cookie,
		include:      include,
		exclude:      exclude,
		stopPruning:  make(chan struct{}),
	}
	// Sessions that are never revoked and ceremonies that are never completed
	// only expire logically, so a sweep keeps both tables bounded.
	go instance.prune()
	replaceRuntime(instance)
	return instance.manager.Middleware(writeUnavailable), nil
}

// replaceRuntime installs instance and releases the runtime it replaces.
// Repeated framework initialization, which tests perform, must not leave an
// earlier sweep running or an earlier backend client open.
func replaceRuntime(instance *runtime) {
	current.Lock()
	defer current.Unlock()
	if previous := current.runtime; previous != nil {
		close(previous.stopPruning)
		if previous.sessionClose != nil {
			_ = previous.sessionClose(context.Background())
		}
	}
	current.runtime = instance
}

// pruneInterval bounds how often expired records are swept.
const pruneInterval = 10 * time.Minute

// pruneBatch bounds one sweep so a large backlog cannot hold connections.
const pruneBatch = 256

func (rt *runtime) prune() {
	ticker := time.NewTicker(pruneInterval)
	defer ticker.Stop()
	for {
		select {
		case <-rt.stopPruning:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			now := time.Now()
			if rt.sessionPrune != nil {
				_, _ = rt.sessionPrune(ctx, now, pruneBatch)
			}
			_, _ = rt.stateStore.Prune(ctx, now, pruneBatch)
			cancel()
		}
	}
}

func setupEndpoints(context.Context) (pw.Middleware, error) {
	instance := activeRuntime()
	if instance == nil {
		return nil, nil
	}
	return instance.endpoints, nil
}

func setupGuard(context.Context) (pw.Middleware, error) {
	instance := activeRuntime()
	if instance == nil || len(instance.include) == 0 {
		return nil, nil
	}
	return instance.guard, nil
}

// closeRuntime stops the expiry sweep and closes an owned backend client
// during shutdown. The database pool belongs to the RDB middleware, so only
// this package's own state is released.
func closeRuntime(context.Context) error {
	replaceRuntime(nil)
	return nil
}

func sessionOptions(config pw.SessionConfig) (session.Options[SessionData], error) {
	sameSite, err := parseSameSite(config.Cookie.SameSite)
	if err != nil {
		return session.Options[SessionData]{}, err
	}
	return session.Options[SessionData]{
		TTL:             config.TTL,
		IdleTimeout:     config.IdleTimeout,
		RenewalInterval: config.RenewalInterval,
		Cookie: session.CookieOptions{
			Name:     config.Cookie.Name,
			Path:     config.Cookie.Path,
			Domain:   config.Cookie.Domain,
			Secure:   config.Cookie.Secure,
			HTTPOnly: config.Cookie.HTTPOnly,
			SameSite: sameSite,
		},
		Method:  methodOIDC,
		Subject: func(data SessionData) string { return data.AccountID },
	}, nil
}

func parseSameSite(value string) (http.SameSite, error) {
	switch value {
	case "", "lax":
		return http.SameSiteLaxMode, nil
	case "strict":
		return http.SameSiteStrictMode, nil
	case "none":
		return http.SameSiteNoneMode, nil
	default:
		return 0, fmt.Errorf("session.cookie.same_site must be strict, lax, or none, got %q", value)
	}
}

func writeUnavailable(w http.ResponseWriter, r *http.Request, err error) {
	pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "session backend unavailable", pw.Err(err))
	pw.WriteProblem(w, r, pw.ServiceUnavailable())
}
