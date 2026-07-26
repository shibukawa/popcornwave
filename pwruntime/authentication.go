package pwruntime

import (
	"context"
	"time"
)

// Authentication is the immutable, protocol-neutral authentication result that
// middleware records for downstream handlers and authorization checks. The zero
// value is an explicitly unauthenticated request.
//
// It never carries passwords, token bodies, cookie values, or provider secrets.
type Authentication struct {
	Authenticated bool
	// Subject is the stable local identity identifier.
	Subject string
	// Method is session, oidc, passkey, bearer, or an application-defined name.
	Method string
	// Principal is an optional application-defined typed value. Middleware must
	// freeze or copy mutable claims before installing it.
	Principal any
	// Scope carries optional tenant, role, or permission values.
	Scope           []string
	AuthenticatedAt time.Time
	ExpiresAt       time.Time
}

// SessionView is the safe request view of a stored session record. It excludes
// the raw cookie token, the store key hash, and the CSRF secret.
type SessionView struct {
	// Data is the typed application session payload. Session accessors return
	// it through a typed assertion instead of exposing this field directly.
	Data      any
	CreatedAt time.Time
	// AuthenticatedAt records the last authentication-strength change.
	AuthenticatedAt time.Time
	LastSeenAt      time.Time
	ExpiresAt       time.Time
	IdleExpiresAt   time.Time
	Method          string
	Version         int
}

// Authentication reports the verified authentication state of the request. A
// request without authentication middleware reports the unauthenticated zero
// value.
func RequestAuthentication(ctx context.Context) Authentication {
	return resources(ctx).Authentication
}

// Session reports the validated session view installed by session middleware.
func Session(ctx context.Context) (SessionView, bool) {
	view := resources(ctx).Session
	if view == nil {
		return SessionView{}, false
	}
	return *view, true
}

// WithAuthentication installs the verified authentication result while
// preserving every other runtime resource already present on ctx. Only
// framework authentication middleware calls it.
func WithAuthentication(ctx context.Context, authentication Authentication) context.Context {
	current := *resources(ctx)
	current.Authentication = authentication
	return WithResources(ctx, current)
}

// WithSession installs the validated session view while preserving every other
// runtime resource already present on ctx. Passing nil clears the view.
func WithSession(ctx context.Context, view *SessionView) context.Context {
	current := *resources(ctx)
	current.Session = view
	return WithResources(ctx, current)
}
