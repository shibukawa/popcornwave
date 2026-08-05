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

// Authentication reports the verified authentication state of the request. A
// request without authentication middleware reports the unauthenticated zero
// value.
func RequestAuthentication(ctx context.Context) Authentication {
	return resources(ctx).Authentication
}

// WithAuthentication installs the verified authentication result while
// preserving every other runtime resource already present on ctx. Only
// framework authentication middleware calls it.
func WithAuthentication(ctx context.Context, authentication Authentication) context.Context {
	current := *resources(ctx)
	current.Authentication = authentication
	return WithResources(ctx, current)
}
