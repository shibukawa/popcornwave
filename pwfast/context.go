package pwfast

import (
	"context"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// Config returns the resolved configuration for T.
//
// It is the same two-step answer the other half gives, from the same shared
// resolution: whatever a middleware recorded on this request, and otherwise the
// process-wide parsed value.
//
// The parameter is a context rather than the request because this transport's
// request value is one. That is the property that made the request-scoped
// accessors portable at all — a handler passing r where a context is wanted
// compiles unchanged on both sides — so these entries take ctx and a rewritten
// call needs no argument moved.
//
// Nothing here parses a configuration file. This half binds none, so what it
// reads is what the runtime that did the binding published. A build with no
// such runtime in it reads zero values, which is the honest answer rather than
// a guess: no file was read, so no setting was chosen.
func Config[T any](ctx context.Context) T {
	return pwruntime.ResolveConfig[T](ctx)
}

// Logger returns the request's logger, or the process default.
func Logger(ctx context.Context) pwruntime.Logger {
	return pwruntime.ReadLogger(ctx)
}

// Authentication is the verified authentication result recorded by
// authentication middleware.
type Authentication = pwruntime.Authentication

// RequestAuthentication returns the verified authentication result of the
// request. A request without authentication middleware, or an anonymous
// request, reports the explicitly unauthenticated zero value.
//
// Authorization must consume this value, never the presence of a cookie.
func RequestAuthentication(ctx context.Context) Authentication {
	return pwruntime.RequestAuthentication(ctx)
}

// Authenticated reports whether the request carries a verified identity.
func Authenticated(ctx context.Context) bool {
	return pwruntime.RequestAuthentication(ctx).Authenticated
}
