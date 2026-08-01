package pw

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/shibukawa/popcornwave/session"
)

// SessionResources are the framework resources a session backend may borrow.
// A backend closes nothing it finds here: what it did not open, it does not
// own.
type SessionResources struct {
	// DB is the pool of api:rdb-middleware, already pinned to the session
	// connection group. It is nil when no database is configured.
	DB *sql.DB
	// DBDriver is the driver scheme of DB, for a backend whose SQL is dialect
	// specific.
	DBDriver string
}

// SessionBackendFactory opens one storage backend from configuration. It reads
// only the keys of its own backend, opens and validates its own dependencies,
// and returns them with the store so the host can release them later.
//
// Registration is a package init; opening happens here, at startup.
type SessionBackendFactory func(context.Context, SessionConfig, SessionResources) (session.Backend, error)

var sessionBackends struct {
	sync.RWMutex
	factories map[string]SessionBackendFactory
}

// knownSessionBackendImports maps a backend nobody registered to the import
// that would. A missing storage plugin is the one configuration mistake whose
// fix is a single line, so the startup error prints that line instead of a
// list of names.
var knownSessionBackendImports = map[string]string{
	SessionBackendRDB:    "github.com/shibukawa/popcornwave/sessionstore/sqlite",
	SessionBackendRedis:  "github.com/shibukawa/popcornwave/sessionstore/redis",
	SessionBackendDynamo: "github.com/shibukawa/popcornwave/sessionstore/dynamo",
}

// RegisterSessionBackend registers factory under name. A storage plugin calls
// it from init, so a blank import is what puts a backend in a binary:
//
//	import _ "github.com/shibukawa/popcornwave/sessionstore/redis"
//
// The cookie backend is the exception. It stores records in the browser and
// adds no dependency, so pw registers it here and it needs no import.
//
// A duplicate or empty name panics: two backends answering one configuration
// value is a build mistake, not a runtime condition.
func RegisterSessionBackend(name string, factory SessionBackendFactory) {
	if name == "" || factory == nil {
		panic("pw: session backend needs a name and a factory")
	}
	sessionBackends.Lock()
	defer sessionBackends.Unlock()
	if sessionBackends.factories == nil {
		sessionBackends.factories = make(map[string]SessionBackendFactory)
	}
	if _, taken := sessionBackends.factories[name]; taken {
		panic(fmt.Sprintf("pw: session backend %q is already registered", name))
	}
	sessionBackends.factories[name] = factory
}

// SessionBackends lists the registered backend names in order. It is what the
// startup summary and error messages report.
func SessionBackends() []string {
	sessionBackends.RLock()
	defer sessionBackends.RUnlock()
	names := make([]string, 0, len(sessionBackends.factories))
	for name := range sessionBackends.factories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// OpenSessionBackend opens the backend named by config.Backend.
//
// A host calls this instead of importing a storage plugin, so adding a backend
// changes no host and links nothing into an application that did not ask for
// it.
func OpenSessionBackend(ctx context.Context, config SessionConfig, resources SessionResources) (session.Backend, error) {
	name := strings.TrimSpace(config.Backend)
	if name == "" {
		name = SessionBackendCookie
	}
	sessionBackends.RLock()
	factory, ok := sessionBackends.factories[name]
	sessionBackends.RUnlock()
	if !ok {
		return session.Backend{}, missingSessionBackend(name)
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

// missingSessionBackend explains an unregistered backend in terms of what the
// application can do about it.
func missingSessionBackend(name string) error {
	if path, known := knownSessionBackendImports[name]; known {
		return fmt.Errorf(
			"session.backend = %q needs its plugin; add to the application: import _ %q", name, path)
	}
	return fmt.Errorf("session.backend = %q is not registered; registered backends: %s",
		name, strings.Join(SessionBackends(), ", "))
}

// SessionCookiePolicy resolves the validated browser cookie policy of the
// session middleware. The cookie backend and the session manager share it, so
// both halves of a session travel under one policy.
func SessionCookiePolicy(config SessionConfig) (session.CookieOptions, error) {
	sameSite, err := parseSessionSameSite(config.Cookie.SameSite)
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
	}, nil
}

func parseSessionSameSite(value string) (http.SameSite, error) {
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
	RegisterSessionBackend(SessionBackendCookie, openCookieSessionBackend)
}

// openCookieSessionBackend builds the built-in browser backend. It opens
// nothing, so it hands back neither a Close nor a Prune.
func openCookieSessionBackend(_ context.Context, config SessionConfig, _ SessionResources) (session.Backend, error) {
	keys, err := sessionCookieKeyring(config.CookieStore)
	if err != nil {
		return session.Backend{}, err
	}
	policy, err := SessionCookiePolicy(config)
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

// sessionCookieKeyring reads the secret that seals cookie-backed records. The
// secret itself never reaches an error message or a log.
func sessionCookieKeyring(config SessionCookieStoreConfig) (*session.Keyring, error) {
	if strings.TrimSpace(config.Secret) == "" {
		return nil, errors.New(
			`session.backend = "cookie" requires session.cookie_store.secret; generate one with: openssl rand -base64 32`)
	}
	secrets := append([]string{config.Secret}, config.PreviousSecrets...)
	keys, err := session.ParseKeyring(secrets...)
	if err != nil {
		return nil, fmt.Errorf("session.cookie_store.secret: %w", err)
	}
	return keys, nil
}
