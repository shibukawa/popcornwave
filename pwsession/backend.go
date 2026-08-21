package pwsession

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/shibukawa/popcornweb/pwconfig"
	"github.com/shibukawa/popcornweb/session"
	"github.com/shibukawa/popcornweb/sessionconfig"
	"github.com/shibukawa/tinybind-go/sqlbind"
)

// Resources are the framework resources a session backend may borrow.
// A backend closes nothing it finds here: what it did not open, it does not
// own.
type Resources struct {
	// DB is the pool of api:rdb-middleware, already pinned to the session
	// connection group. It is nil when no database is configured, and also on
	// an engine that bypasses database/sql; Executor is the surface that
	// exists on every connection.
	DB *sql.DB
	// Executor is the statement surface of the session connection group,
	// whichever kind of pool backs it. It is nil when no database is
	// configured.
	Executor sqlbind.SQLExecutor
	// DBDriver is the driver scheme of the connection, for a backend whose
	// SQL is dialect specific.
	DBDriver string
}

// BackendFactory opens one storage backend from configuration. It reads
// only the keys of its own backend, opens and validates its own dependencies,
// and returns them with the store so the host can release them later.
//
// Registration is a package init; opening happens here, at startup.
type BackendFactory func(context.Context, sessionconfig.SessionConfig, Resources) (session.Backend, error)

var backendState struct {
	sync.RWMutex
	factories map[string]BackendFactory
}

// knownBackendImports maps a backend nobody registered to the import
// that would. A missing storage plugin is the one configuration mistake whose
// fix is a single line, so the startup error prints that line instead of a
// list of names.
var knownBackendImports = map[string]string{
	sessionconfig.SessionBackendRDB:       "github.com/shibukawa/popcornweb/sessionstore/sqlite",
	sessionconfig.SessionBackendRedis:     "github.com/shibukawa/popcornweb/sessionstore/redis",
	sessionconfig.SessionBackendDynamo:    "github.com/shibukawa/popcornweb/sessionstore/dynamo",
	sessionconfig.SessionBackendFirestore: "github.com/shibukawa/popcornweb/sessionstore/firestore",
}

// RegisterBackend registers factory under name. A storage plugin calls
// it from init, so a blank import is what puts a backend in a binary:
//
//	import _ "github.com/shibukawa/popcornweb/sessionstore/redis"
//
// Cookie and the two development intent modes are built in. They add no
// storage dependency, so pw registers them here and they need no import.
//
// A duplicate or empty name panics: two backends answering one configuration
// value is a build mistake, not a runtime condition.
func RegisterBackend(name string, factory BackendFactory) {
	if name == "" || factory == nil {
		panic("pw: session backend needs a name and a factory")
	}
	backendState.Lock()
	defer backendState.Unlock()
	if backendState.factories == nil {
		backendState.factories = make(map[string]BackendFactory)
	}
	if _, taken := backendState.factories[name]; taken {
		panic(fmt.Sprintf("pw: session backend %q is already registered", name))
	}
	backendState.factories[name] = factory
}

// Backends lists the registered backend names in order. It is what the
// startup summary and error messages report.
func Backends() []string {
	backendState.RLock()
	defer backendState.RUnlock()
	names := make([]string, 0, len(backendState.factories))
	for name := range backendState.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// OpenBackend opens the backend named by config.Backend.
//
// A host calls this instead of importing a storage plugin, so adding a backend
// changes no host and links nothing into an application that did not ask for
// it.
func OpenBackend(ctx context.Context, config sessionconfig.SessionConfig, resources Resources) (session.Backend, error) {
	name := strings.TrimSpace(config.Backend)
	if name == "" {
		name = sessionconfig.SessionBackendCookie
	}
	backendState.RLock()
	factory, ok := backendState.factories[name]
	backendState.RUnlock()
	if !ok {
		return session.Backend{}, missingBackend(name)
	}
	backend, err := factory(ctx, config, resources)
	if err != nil {
		return session.Backend{}, err
	}
	if backend.Store == nil {
		return session.Backend{}, fmt.Errorf("session.backend %q returned no store", name)
	}
	return backend, nil
}

// missingBackend explains an unregistered backend in terms of what the
// application can do about it.
func missingBackend(name string) error {
	if path, known := knownBackendImports[name]; known {
		return fmt.Errorf(
			"session.backend = %q needs its plugin; add to the application: import _ %q", name, path)
	}
	return fmt.Errorf("session.backend = %q is not registered; registered backends: %s",
		name, strings.Join(Backends(), ", "))
}

// CookiePolicy resolves the validated browser cookie policy of the
// session middleware. The cookie backend and the session manager share it, so
// both halves of a session travel under one policy.
func CookiePolicy(config sessionconfig.SessionConfig) (session.CookieOptions, error) {
	sameSite, err := parseSameSite(config.Cookie.SameSite)
	if err != nil {
		return session.CookieOptions{}, err
	}
	return session.CookieOptions{
		Name:     config.Cookie.Name,
		Path:     config.Cookie.Path,
		Domain:   config.Cookie.Domain,
		Secure:   config.Cookie.Secure,
		HTTPOnly: config.Cookie.HTTPOnly,
		SameSite: sameSite,
		// The configuration schema defaults both attributes to true, so a false
		// arriving here was written by the deployment and stands as written.
		AllowInsecure:  !config.Cookie.Secure,
		ScriptReadable: !config.Cookie.HTTPOnly,
	}, nil
}

// ParseSameSite reads the configured attribute, which the startup validation
// also needs: "none" without Secure is a cookie no browser accepts, and that is
// refused before a port is bound rather than at the first response.
func ParseSameSite(value string) (http.SameSite, error) { return parseSameSite(value) }

func parseSameSite(value string) (http.SameSite, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "lax":
		return http.SameSiteLaxMode, nil
	case "strict":
		return http.SameSiteStrictMode, nil
	case "none":
		return http.SameSiteNoneMode, nil
	default:
		return 0, fmt.Errorf("session.cookie.same_site %q must be strict, lax, or none", value)
	}
}

func init() {
	RegisterBackend(sessionconfig.SessionBackendCookie, openCookieBackend)
	RegisterBackend(sessionconfig.SessionBackendDevVolatile, openDevVolatileBackend)
	RegisterBackend(sessionconfig.SessionBackendDevPersist, openDevPersistBackend)
}

func ValidateDevelopmentMode(name string) error {
	if (name == sessionconfig.SessionBackendDevVolatile || name == sessionconfig.SessionBackendDevPersist) && !pwconfig.Development() {
		return fmt.Errorf("session.backend = %q is available only when APP_ENV=dev", name)
	}
	return nil
}

// openDevVolatileBackend builds the process-local development backend.
func openDevVolatileBackend(_ context.Context, config sessionconfig.SessionConfig, _ Resources) (session.Backend, error) {
	if err := ValidateDevelopmentMode(config.Backend); err != nil {
		return session.Backend{}, err
	}
	return session.Backend{Store: session.NewMemoryStore(nil)}, nil
}

// openDevPersistBackend exposes the cookie store under a name that
// states its development restart behavior. setupSession uses its ordinary
// cookie path so the manager constructs exactly one browser store.
func openDevPersistBackend(ctx context.Context, config sessionconfig.SessionConfig, resources Resources) (session.Backend, error) {
	if err := ValidateDevelopmentMode(config.Backend); err != nil {
		return session.Backend{}, err
	}
	return openCookieBackend(ctx, config, resources)
}

// openCookieBackend builds the built-in browser backend. It opens
// nothing, so it hands back neither a Close nor a Prune.
func openCookieBackend(_ context.Context, config sessionconfig.SessionConfig, _ Resources) (session.Backend, error) {
	keys, err := cookieKeyring(config.Backend, config.Keyring)
	if err != nil {
		return session.Backend{}, err
	}
	policy, err := CookiePolicy(config)
	if err != nil {
		return session.Backend{}, err
	}
	// The record travels with the token, so it repeats the token's policy and
	// only takes its own name.
	policy.Name = config.CookieStore.Name
	store, err := session.NewCookieStore(session.CookieStoreOptions{Keys: keys, Cookie: policy})
	if err != nil {
		return session.Backend{}, fmt.Errorf("session.cookie_store: %w", err)
	}
	return session.Backend{Store: store}, nil
}

// cookieKeyring reads the secret that seals cookie-backed records. The
// secret itself never reaches an error message or a log.
func cookieKeyring(backend string, config sessionconfig.SessionKeyringConfig) (*session.Keyring, error) {
	if strings.TrimSpace(config.Secret) == "" {
		if backend == "" {
			backend = sessionconfig.SessionBackendCookie
		}
		return nil, fmt.Errorf(
			"session.backend = %q requires session.keyring.secret; generate one with: openssl rand -base64 32", backend)
	}
	secrets := append([]string{config.Secret}, config.PreviousSecrets...)
	keys, err := session.ParseKeyring(secrets...)
	if err != nil {
		return nil, fmt.Errorf("session.keyring.secret: %w", err)
	}
	return keys, nil
}
