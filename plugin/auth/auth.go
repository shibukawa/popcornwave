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
// correlation state stored by authstate/sqlite.
//
// This package imports no storage plugin. It asks pw for the configured
// backend, so an application links the storage it configured and no more, and
// a backend other than cookie needs its own blank import.
package auth

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/authstate"
	"github.com/shibukawa/popcornwave/contrib/oauth"
	"github.com/shibukawa/popcornwave/contrib/oidc"
	"github.com/shibukawa/popcornwave/contrib/passkey"
	"github.com/shibukawa/popcornwave/internal/pathpattern"
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
	registerSessionSlot()
	pw.RegisterExtension(pw.Extension{
		Name:  "auth.endpoints",
		Slot:  pw.SlotAuthentication,
		Setup: setupAuthentication,
		Close: closeRuntime,
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
	manager *session.Manager
	// sessionClose releases a client the backend opened. A backend that
	// borrowed the middleware database leaves it nil.
	sessionClose func(context.Context) error
	backend      Backend
	// pruners are the ceremony stores that need sweeping, which is none of
	// them on a backend whose expiry is decided on read.
	pruners    []statePruner
	stateStore authstate.Store[oauth.Transaction]
	// hint is the sealed sign-in hint cookie, nil unless the deployment turned
	// it on. It carries no authority; see SignInHint.
	hint         *session.Jar[SignInHint]
	allowlist    AllowlistStore
	cookiePolicy pw.SessionCookieConfig
	include      []pathpattern.Pattern
	exclude      []pathpattern.Pattern
	// passkeyFlow is nil unless the selected mode mounts api:passkey-endpoints.
	passkeyFlow *passkey.SessionFlow
	// credentials and bootstrap are the installed stores, or the framework
	// defaults over the tables this package owns.
	credentials CredentialStore
	bootstrap   BootstrapStore
	// enrollment holds the restricted tickets a redeemed bootstrap credential
	// grants. It is nil outside passkey_only.
	enrollment authstate.Store[enrollmentTicket]
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

// setupAuthentication validates configuration, opens the state this package
// owns, and returns the middleware that finalizes the request authentication
// and serves the login endpoints.
//
// It runs at SlotAuthentication, after the framework has already resolved the
// session at SlotSession. Storage is not this package's job: it reaches the
// manager pw prepared and drives it.
func setupAuthentication(ctx context.Context) (pw.Middleware, error) {
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
	manager := pw.SessionManager()
	if manager == nil {
		return nil, errors.New("auth requires session.enabled = true")
	}
	// The stores this package owns come from the selected backend. Only the
	// relational one needs a database handle, and it is the one that says so:
	// a project on DynamoDB reaches none of this.
	resources, err := backendResources(ctx)
	if err != nil {
		return nil, err
	}
	schemaCtx, cancel := context.WithTimeout(ctx, schemaTimeout)
	defer cancel()
	backend, err := openBackend(schemaCtx, config, resources)
	if err != nil {
		return nil, err
	}
	include, err := pathpattern.Compile(config.Protection.Include)
	if err != nil {
		return nil, err
	}
	exclude, err := pathpattern.Compile(config.Protection.Exclude)
	if err != nil {
		return nil, err
	}
	instance := &runtime{
		config:       config,
		manager:      manager,
		backend:      backend,
		allowlist:    backend.Allowlist,
		cookiePolicy: sessionConfig.Cookie,
		include:      include,
		exclude:      exclude,
		stopPruning:  make(chan struct{}),
		passkeyPaths: config.passkeyPaths(),
	}
	if instance.stateStore, err = openState(schemaCtx, instance, stateNamespace, oauth.TransactionCodec{}); err != nil {
		return nil, err
	}
	if instance.hint, err = hintJar(config.Assurance.Hint, sessionConfig.Cookie); err != nil {
		return nil, err
	}
	if config.usesPasskey() {
		if err := instance.setupPasskey(schemaCtx); err != nil {
			return nil, err
		}
	}
	// Ceremonies that are never completed only expire logically, so a sweep
	// keeps the table bounded. A backend whose store needs none registers no
	// pruner and the goroutine idles.
	go instance.prune()
	replaceRuntime(instance)
	return instance.authenticate, nil
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
			// The session store sweeps itself; these are the ceremony records
			// this package owns.
			for _, pruner := range rt.pruners {
				_, _ = pruner.Prune(ctx, now, pruneBatch)
			}
			cancel()
		}
	}
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

func writeUnavailable(w http.ResponseWriter, r *http.Request, err error) {
	pw.Logger(r.Context()).Log(r.Context(), pw.LevelError, "session backend unavailable", pw.Err(err))
	pw.WriteProblem(w, r, pw.ServiceUnavailable())
}
