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

// DB returns the pool of the effective connection group.
func DB(ctx context.Context) (*sql.DB, bool) { return pwruntime.DB(ctx) }

// DBDriver reports the driver scheme of the effective framework database pool.
func DBDriver(ctx context.Context) (string, bool) { return pwruntime.DBDriver(ctx) }

// SelectDB pins a connection group onto ctx, so generated SQL and Transaction
// run against that group instead of the default one:
//
//	CreateUser(pw.SelectDB(ctx, "writer"), options)
//
// An unknown group name fails at the first statement that uses the returned
// context, not here.
func SelectDB(ctx context.Context, group string) context.Context {
	return pwruntime.SelectDB(ctx, group)
}

// TxOption customizes one Transaction call.
type TxOption = pwruntime.TxOption

// OnGroup runs a transaction against a named connection group.
func OnGroup(group string) TxOption { return pwruntime.OnGroup(group) }

// Transaction runs fn inside a database transaction and passes it a context
// whose generated SQL functions use that transaction. A nested call opens a
// savepoint instead of a new transaction, so its failure rolls back only its
// own work and the outer transaction stays usable.
//
// Without OnGroup the transaction runs on the effective group of ctx, so
// unpinned SQL inside it stays on that group rather than falling back to the
// default one:
//
//	pw.Transaction(ctx, func(ctx context.Context) error { ... }, pw.OnGroup("writer"))
func Transaction(ctx context.Context, fn func(context.Context) error, options ...TxOption) error {
	return pwruntime.Transaction(ctx, fn, options...)
}
