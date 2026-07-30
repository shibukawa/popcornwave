// Package auth adds session-backed browser authentication to a Popcorn Wave
// application. Importing it registers the [auth] configuration binding and the
// framework extensions that resolve sessions, own the login endpoints, and
// guard protected paths.
//
//	import _ "github.com/shibukawa/popcornwave/plugin/auth"
//
// The current build implements auth.mode = "oidc_only": OpenID Connect
// Authorization Code with PKCE against one configured issuer, a login session
// stored by plugin/session/rdb, and single-use OAuth correlation state stored
// by contrib/authstate/sqlite.
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
	"github.com/shibukawa/popcornwave/contrib/passkey"
	"github.com/shibukawa/popcornwave/plugin/session/rdb"
	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/session"
)

// Authentication method names recorded on a session and reported by
// pw.RequestAuthentication. An application compares against these rather than
// against a literal.
const (
	// MethodOIDC labels sessions created by the OIDC flow.
	MethodOIDC = "oidc"
	// MethodPasskey labels sessions created by a passkey assertion.
	MethodPasskey = "passkey"
)

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
	config       Config
	manager      *session.Manager[SessionData]
	sessionStore *rdb.Store[SessionData]
	stateStore   *authsqlite.Store[oauth.Transaction]
	allowlist    Allowlist
	cookiePolicy pw.SessionCookieConfig
	include      []pattern
	exclude      []pattern
	// passkeyFlow is nil unless the selected mode mounts api:passkey-endpoints.
	passkeyFlow *passkey.SessionFlow
	// credentials and bootstrap are the installed stores, or the framework
	// defaults over the tables this package owns.
	credentials CredentialStore
	bootstrap   BootstrapStore
	// enrollment holds the restricted tickets a redeemed bootstrap credential
	// grants. It is nil outside passkey_only.
	enrollment *authsqlite.Store[enrollmentTicket]
	// passkeyPaths maps a mounted ceremony path to its endpoint suffix.
	passkeyPaths map[string]string
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
	if sessionConfig.Backend != "rdb" {
		return nil, fmt.Errorf("session.backend %q is not implemented; use \"rdb\"", sessionConfig.Backend)
	}
	if sessionConfig.RDB.Source != "middleware" {
		return nil, fmt.Errorf("session.rdb.source %q is not implemented; use \"middleware\"", sessionConfig.RDB.Source)
	}
	if _, ok := pw.DB(ctx); !ok {
		return nil, errors.New("session.rdb.source = \"middleware\" requires middleware.rdb.enabled = true")
	}
	// The session table is written on every login, so it lives in the session
	// group rather than in the default group, which is normally a replica.
	sessionCtx, err := pw.SelectSessionDB(ctx)
	if err != nil {
		return nil, err
	}
	db, ok := pw.DB(sessionCtx)
	if !ok {
		return nil, errors.New("session.rdb.source = \"middleware\" requires middleware.rdb.enabled = true")
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
	sessionStore, err := rdb.NewStore[SessionData](db, session.JSONCodec[SessionData]{}, rdb.Options{
		Table: sessionConfig.RDB.Table,
	})
	if err != nil {
		return nil, err
	}
	stateStore, err := authsqlite.NewStore[oauth.Transaction](db, oauth.TransactionCodec{}, authsqlite.Options{
		Namespace: stateNamespace,
	})
	if err != nil {
		return nil, err
	}
	// Framework tables are migration-owned, so startup verifies them instead
	// of creating them.
	schemaCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	table := sessionConfig.RDB.Table
	if table == "" {
		table = rdb.DefaultTable
	}
	if err := verifyTables(schemaCtx, db, table, config); err != nil {
		return nil, err
	}
	if err := sessionStore.VerifySchema(schemaCtx); err != nil {
		return nil, fmt.Errorf("session schema: %w", err)
	}
	// The table already exists, so this validates its column layout only.
	if err := stateStore.EnsureSchema(schemaCtx); err != nil {
		return nil, fmt.Errorf("auth state schema: %w", err)
	}
	manager, err := session.NewManager[SessionData](sessionStore, options)
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
		sessionStore: sessionStore,
		stateStore:   stateStore,
		allowlist:    Allowlist{db: db},
		cookiePolicy: sessionConfig.Cookie,
		include:      include,
		exclude:      exclude,
		stopPruning:  make(chan struct{}),
		passkeyPaths: config.passkeyPaths(),
	}
	if config.usesPasskey() {
		if err := instance.setupPasskey(schemaCtx, db); err != nil {
			return nil, err
		}
	}
	// Sessions that are never revoked and ceremonies that are never completed
	// only expire logically, so a sweep keeps both tables bounded.
	go instance.prune()
	replaceRuntime(instance)
	return instance.manager.Middleware(writeUnavailable), nil
}

// replaceRuntime installs instance and stops the sweep of the runtime it
// replaces. Repeated framework initialization, which tests perform, must not
// leave an earlier sweep running.
func replaceRuntime(instance *runtime) {
	current.Lock()
	defer current.Unlock()
	if current.runtime != nil {
		close(current.runtime.stopPruning)
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
			_, _ = rt.sessionStore.Prune(ctx, now, pruneBatch)
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

// closeRuntime stops the expiry sweep during shutdown. The database pool
// belongs to the RDB middleware, so only this package's own state is released.
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
		Method:  MethodOIDC,
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
