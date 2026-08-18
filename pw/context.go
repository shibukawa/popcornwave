package pw

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/shibukawa/popcornweb/pwruntime"
)

// Context returns the request's context.Context.
//
// It is the supported way to cross from the request handle to the currency the
// layers below a handler take. r.Context() yields the same value and is legal,
// but it names the net/http request type, which is what a second transport
// cannot follow; this call is rewritten instead.
func Context(r *http.Request) context.Context { return r.Context() }

// Config returns one immutable typed binding of the running configuration.
func Config[T any](r *http.Request) T { return pwruntime.ResolveConfig[T](r.Context()) }

// ConfigContext is Config for code below the handler, which holds a context
// rather than the request.
func ConfigContext[T any](ctx context.Context) T {
	return pwruntime.ResolveConfig[T](ctx)
}

// Authentication is the verified authentication result recorded by
// authentication middleware.
type Authentication = pwruntime.Authentication

// RequestAuthentication returns the verified authentication result of the
// request. A request without authentication middleware, or an anonymous
// request, reports the explicitly unauthenticated zero value.
//
// Authorization must consume this value, never the presence of a cookie.
func RequestAuthentication(r *http.Request) Authentication {
	return pwruntime.RequestAuthentication(r.Context())
}

// RequestAuthenticationContext is RequestAuthentication for code below the
// handler.
func RequestAuthenticationContext(ctx context.Context) Authentication {
	return pwruntime.RequestAuthentication(ctx)
}

// Authenticated reports whether the request carries a verified identity.
func Authenticated(r *http.Request) bool {
	return pwruntime.RequestAuthentication(r.Context()).Authenticated
}

// AuthenticatedContext is Authenticated for code below the handler.
func AuthenticatedContext(ctx context.Context) bool {
	return pwruntime.RequestAuthentication(ctx).Authenticated
}

// DB returns the pool of the effective connection group.
//
// An engine that bypasses database/sql — PostgreSQL runs on its native pgx
// pool — has no *sql.DB, so DB reports false there. Generated SQL and
// Transaction are the portable surfaces; they run on either kind of pool. A
// raw statement that must run on both resolves the request executor through
// pwruntime.SQLExecutor and reads through sqlbind.Query, which dispatches to
// whichever cursor the connection provides.
func DB(r *http.Request) (*sql.DB, bool) { return pwruntime.DB(r.Context()) }

// DBContext is DB for code below the handler, and for a caller holding a
// context SelectDB has pinned.
func DBContext(ctx context.Context) (*sql.DB, bool) { return pwruntime.DB(ctx) }

// DBDriver reports the driver scheme of the effective framework database pool.
func DBDriver(r *http.Request) (string, bool) { return pwruntime.DBDriver(r.Context()) }

// DBDriverContext is DBDriver for code below the handler.
func DBDriverContext(ctx context.Context) (string, bool) { return pwruntime.DBDriver(ctx) }

// SelectDB pins a connection group, so generated SQL and Transaction run
// against that group instead of the default one:
//
//	CreateUser(pw.SelectDB(r, "writer"), options)
//
// It returns a context rather than a request: pinning is a property of the
// value the layers below the handler carry, and those layers take a context.
//
// An unknown group name fails at the first statement that uses the returned
// context, not here.
func SelectDB(r *http.Request, group string) context.Context {
	return pwruntime.SelectDB(r.Context(), group)
}

// SelectDBContext is SelectDB for code below the handler, including a caller
// re-pinning a context it was given.
func SelectDBContext(ctx context.Context, group string) context.Context {
	return pwruntime.SelectDB(ctx, group)
}

// Transaction runs fn inside a database transaction and passes it a context
// whose generated SQL functions use that transaction. A nested call opens a
// savepoint instead of a new transaction, so its failure rolls back only its
// own work and the outer transaction stays usable.
//
// The transaction runs on the effective group of the request, which SelectDB
// pins for a transaction and for a single statement alike, and unpinned SQL
// inside it stays on that group rather than falling back to the default one:
//
//	pw.TransactionContext(pw.SelectDB(r, "writer"), func(ctx context.Context) error { ... })
func Transaction(r *http.Request, fn func(context.Context) error) error {
	return pwruntime.Transaction(r.Context(), fn)
}

// TransactionContext is Transaction for code below the handler, and for a
// caller holding a context pinned to a group.
func TransactionContext(ctx context.Context, fn func(context.Context) error) error {
	return pwruntime.Transaction(ctx, fn)
}
