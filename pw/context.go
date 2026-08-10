package pw

import (
	"context"
	"database/sql"
	"reflect"

	"github.com/shibukawa/popcornwave/pwruntime"
)

func Config[T any](ctx context.Context) T {
	return pwruntime.ResolveConfig[T](ctx)
}

// The resolved configuration is published for the other runtime, which binds
// none of its own. It is an init rather than a step inside ParseConfig because
// the closure reads the registry when it is called rather than capturing it, so
// publishing it before anything is parsed is both correct and one less ordering
// constraint.
func init() {
	pwruntime.PublishConfigLookup(func(target reflect.Type) (any, bool) {
		configState.RLock()
		defer configState.RUnlock()
		entry, ok := configState.entries[target]
		if !ok {
			return nil, false
		}
		return entry.ptr, true
	})
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

// DB returns the pool of the effective connection group.
//
// An engine that bypasses database/sql — PostgreSQL runs on its native pgx
// pool — has no *sql.DB, so DB reports false there. Generated SQL and
// Transaction are the portable surfaces; they run on either kind of pool. A
// raw statement that must run on both resolves the request executor through
// pwruntime.SQLExecutor and reads through sqlbind.Query, which dispatches to
// whichever cursor the connection provides.
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

// Transaction runs fn inside a database transaction and passes it a context
// whose generated SQL functions use that transaction. A nested call opens a
// savepoint instead of a new transaction, so its failure rolls back only its
// own work and the outer transaction stays usable.
//
// The transaction runs on the effective group of ctx, which SelectDB pins for a
// transaction and for a single statement alike, and unpinned SQL inside it
// stays on that group rather than falling back to the default one:
//
//	pw.Transaction(pw.SelectDB(ctx, "writer"), func(ctx context.Context) error { ... })
func Transaction(ctx context.Context, fn func(context.Context) error) error {
	return pwruntime.Transaction(ctx, fn)
}
