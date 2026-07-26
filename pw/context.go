package pw

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/shibukawa/popcornwave/pwruntime"
)

func Config[T any](ctx context.Context) T {
	if value, ok := pwruntime.Config[T](ctx); ok {
		return value
	}
	if value, ok := registeredConfig[T](); ok {
		return value
	}
	var zero T
	return zero
}

func Logger(ctx context.Context) *slog.Logger { return pwruntime.Logger(ctx) }

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

func DB(ctx context.Context) (*sql.DB, bool) { return pwruntime.DB(ctx) }

// DBDriver reports the driver scheme of the framework database pool.
func DBDriver(ctx context.Context) (string, bool) { return pwruntime.DBDriver(ctx) }

// Transaction runs fn inside a database transaction and passes it a context
// whose generated SQL functions use that transaction. A nested call opens a
// savepoint instead of a new transaction, so its failure rolls back only its
// own work and the outer transaction stays usable.
func Transaction(ctx context.Context, fn func(context.Context) error) error {
	return pwruntime.Transaction(ctx, fn)
}
