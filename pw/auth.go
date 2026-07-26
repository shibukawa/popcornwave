package pw

import (
	"context"
	"errors"
	"net/http"
	"sync"
)

// Identity is the authenticated user of the current request. It is what the
// framework proved at login, not an application account record: an application
// resolves its own account from Subject.
type Identity struct {
	// Subject is the provider-issued identifier, unique within Issuer.
	Subject string
	Issuer  string
	Name    string
	Email   string
	// Claims carries the remaining string claims the provider returned.
	Claims map[string]string
}

// Claim returns one additional claim.
func (identity Identity) Claim(name string) (string, bool) {
	value, ok := identity.Claims[name]
	return value, ok
}

type identityContextKey struct{}

// WithIdentity installs the authenticated identity on a request context. It is
// part of the contract with the authentication package and is not needed by
// application code.
func WithIdentity(ctx context.Context, identity Identity) context.Context {
	if ctx == nil {
		return nil
	}
	return context.WithValue(ctx, identityContextKey{}, identity)
}

// CurrentUser returns the authenticated identity of the request, if any.
// Handlers use it to render a signed-in view; it is never a permission check.
func CurrentUser(ctx context.Context) (Identity, bool) {
	if ctx == nil {
		return Identity{}, false
	}
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	return identity, ok
}

// AuthHandlers are the framework-owned authentication endpoints. An
// application registers none of them: the runtime mounts them at the
// AuthConfig paths, so a login exists as soon as the configuration does.
type AuthHandlers struct {
	// Login starts the provider login. GET.
	Login http.Handler
	// Callback completes it and establishes the session. GET.
	Callback http.Handler
	// Logout ends the session. POST only, so a link or a prefetch cannot sign
	// a user out.
	Logout http.Handler
	// Resolve installs the identity of an existing session on every request.
	Resolve func(http.Handler) http.Handler
}

// AuthProvider builds the endpoints for the resolved configuration. The
// authentication package registers one during init.
type AuthProvider func(AuthConfig, SessionConfig) (AuthHandlers, error)

var authProviderState struct {
	sync.RWMutex
	provider AuthProvider
}

// RegisterAuthProvider installs the implementation behind AuthConfig. Importing
// github.com/shibukawa/popcornwave/auth performs this registration; an
// application calls it only to supply its own implementation.
func RegisterAuthProvider(provider AuthProvider) {
	if provider == nil {
		panic("popcornwave: nil authentication provider")
	}
	authProviderState.Lock()
	defer authProviderState.Unlock()
	authProviderState.provider = provider
}

func registeredAuthProvider() AuthProvider {
	authProviderState.RLock()
	defer authProviderState.RUnlock()
	return authProviderState.provider
}

// authRuntime is the configuration pair the endpoints need.
type authRuntime struct {
	auth    AuthConfig
	session SessionConfig
}

// installAuthEndpoints mounts the registered provider. Enabling authentication
// without an implementation is a startup error rather than a set of paths that
// silently return 404.
func installAuthEndpoints(next http.Handler, runtime authRuntime) (http.Handler, error) {
	if !runtime.auth.Enabled {
		return next, nil
	}
	provider := registeredAuthProvider()
	if provider == nil {
		return nil, errors.New(`popcornwave: auth.enabled requires an authentication provider; add _ "github.com/shibukawa/popcornwave/auth" to the application imports`)
	}
	handlers, err := provider(runtime.auth, runtime.session)
	if err != nil {
		return nil, err
	}
	if handlers.Login == nil || handlers.Callback == nil || handlers.Logout == nil {
		return nil, errors.New("popcornwave: authentication provider returned incomplete handlers")
	}
	result := authEndpoints(next, runtime.auth, handlers)
	if handlers.Resolve != nil {
		result = handlers.Resolve(result)
	}
	return result, nil
}

// authEndpoints answers the configured authentication paths and passes
// everything else through.
func authEndpoints(next http.Handler, config AuthConfig, handlers AuthHandlers) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case config.LoginPath:
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				methodNotAllowed(w, r, http.MethodGet)
				return
			}
			handlers.Login.ServeHTTP(w, r)
		case config.CallbackPath:
			if r.Method != http.MethodGet {
				methodNotAllowed(w, r, http.MethodGet)
				return
			}
			handlers.Callback.ServeHTTP(w, r)
		case config.LogoutPath:
			// POST only: a logout that a link or a prefetch can trigger is a
			// denial-of-service surface, not a convenience.
			if r.Method != http.MethodPost {
				methodNotAllowed(w, r, http.MethodPost)
				return
			}
			handlers.Logout.ServeHTTP(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request, allowed string) {
	w.Header().Set("Allow", allowed)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
