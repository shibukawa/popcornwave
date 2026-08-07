// Package pwruntime contains the narrow runtime contract used by generated
// Popcorn Wave code. Handwritten applications should normally import pw.
package pwruntime

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"sync"

	"github.com/shibukawa/tinybind-go/sqlbind"
)

type contextKey struct{}

// Resources is the immutable process/request state installed by pw.
type Resources struct {
	Configs map[reflect.Type]any
	// Log is the process emission policy. A nil backend falls back to a plain
	// stderr text handler so that a request served outside pw still logs.
	Log *LogBackend
	// LogAttributes are the stable request attributes every record carries,
	// such as the request correlation ID.
	LogAttributes []Attribute
	// DB is the pool of the default group. It stays the whole database for a
	// configuration that declares no connection set.
	DB *sql.DB
	// DBDriver is the driver scheme of DSN, used to decide savepoint support.
	DBDriver string
	// Connections is the configured connection set. A nil value means the
	// single DB above is the only database.
	Connections *ConnectionSet
	// Group is the group pinned by SelectDB. Empty selects the default.
	Group string
	// picked memoizes the round-robin choice per group for one request.
	picked *connectionMemo
	// TxScope is the active transaction scope, installed by the framework only.
	TxScope *TransactionScope
	// Query enables development query diagnostics. A nil value leaves the
	// resolved executor undecorated.
	Query *QueryDiagnostics
	// Authentication is the verified request authentication result, finalized
	// by authentication middleware before handler dispatch.
	Authentication Authentication
	// parent is the capsule this one was derived from, nil for the capsule
	// installed at request start. One context lookup reaches the innermost
	// capsule; its ancestors are a pointer chase from here, never a second
	// walk of the context chain.
	parent *Resources
}

// Parent returns the capsule this one was derived from, or nil for the
// request root capsule.
func (r *Resources) Parent() *Resources {
	if r == nil {
		return nil
	}
	return r.parent
}

func WithResources(ctx context.Context, resources Resources) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if resources.Log == nil {
		resources.Log = fallbackBackend()
	}
	// The memo is per request, not per process. InjectResources hands every
	// request the same Resources value, whose memo is nil, so each request
	// builds its own here and every child context inherits that one pointer.
	if resources.picked == nil {
		resources.picked = newConnectionMemo()
	}
	return context.WithValue(ctx, contextKey{}, &resources)
}

// SelectDB pins group onto ctx so generated SQL and Transaction use it instead
// of the default group.
//
// An unknown name is not rejected here. The returned context fails at its first
// executor resolution, DB call, or Transaction with ErrUnknownConnectionGroup.
func SelectDB(ctx context.Context, group string) context.Context {
	current := derive(ctx)
	current.Group = group
	return WithResources(ctx, current)
}

// derive copies the capsule of ctx and records the copied capsule as the
// parent of the copy, so every derived capsule can walk its ancestry without
// touching the context again.
func derive(ctx context.Context) Resources {
	current := resources(ctx)
	child := *current
	child.parent = current
	return child
}

// effectiveGroup is the group a statement on ctx runs against.
//
// The active transaction outranks the default group, so unpinned SQL inside a
// writer transaction stays on the writer instead of leaking to a replica.
func (r *Resources) effectiveGroup() string {
	if r.Group != "" {
		return r.Group
	}
	if r.TxScope.Active() {
		return r.TxScope.Group()
	}
	return r.Connections.DefaultGroup()
}

// connection resolves the pool backing the effective group.
func (r *Resources) connection() (*Connection, error) {
	group := r.effectiveGroup()
	if r.Connections == nil {
		if r.DB == nil {
			return nil, errors.New("popcornwave: database is not available in context")
		}
		return &Connection{DB: r.DB, Driver: r.DBDriver, Group: group, Label: group}, nil
	}
	return r.picked.resolve(r.Connections, group)
}

func resources(ctx context.Context) *Resources {
	if ctx != nil {
		if value, ok := ctx.Value(contextKey{}).(*Resources); ok && value != nil {
			return value
		}
	}
	return &Resources{Log: fallbackBackend()}
}

// fallbackBackend serves a context that never passed through pw, which is what
// a unit test and an unconfigured tool both look like. Info to stderr is the
// least surprising thing to do there, and it is built once rather than per
// lookup because an accessor on the request path must not allocate a handler.
var fallbackBackend = sync.OnceValue(func() *LogBackend {
	return NewLogBackend(LevelInfo, NewSlogSink(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
})

func Config[T any](ctx context.Context) (T, bool) {
	var zero T
	value, ok := resources(ctx).Configs[reflect.TypeFor[T]()]
	if !ok {
		return zero, false
	}
	typed, ok := value.(T)
	return typed, ok
}

// ReadLogger returns a logger bound to the emission policy of ctx, its stable
// request attributes, and the span active on it. It never returns a logger that
// cannot be called: a context with nothing installed still yields a usable one.
//
// Acquire again inside a child span to correlate with that span, because the
// correlation is captured here rather than at each call.
func ReadLogger(ctx context.Context) Logger {
	current := resources(ctx)
	return NewLogger(ctx, current.Log, current.LogAttributes...)
}

// WithLogAttributes adds stable request attributes to every record taken from
// ctx afterwards, preserving the other runtime resources already installed.
func WithLogAttributes(ctx context.Context, attributes ...Attribute) context.Context {
	if len(attributes) == 0 {
		return ctx
	}
	current := derive(ctx)
	current.LogAttributes = mergeAttributes(current.LogAttributes, attributes)
	return WithResources(ctx, current)
}

// WithLogBackend replaces only the emission policy, which is what a test needs
// to capture records without rebuilding every other resource.
func WithLogBackend(ctx context.Context, backend *LogBackend) context.Context {
	current := derive(ctx)
	current.Log = backend
	return WithResources(ctx, current)
}

// DB returns the pool of the effective group, which is the group pinned by
// SelectDB, otherwise the group of an active transaction, otherwise the default
// group.
//
// A connection whose engine bypasses database/sql has no *sql.DB, so DB
// reports false for it rather than fabricating a handle; ConnectionExecutor
// is the surface that exists on every connection.
func DB(ctx context.Context) (*sql.DB, bool) {
	connection, err := resources(ctx).connection()
	if err != nil {
		return nil, false
	}
	return connection.DB, connection.DB != nil
}

// ConnectionExecutor returns the pool-level statement surface of the effective
// group, undecorated and outside any transaction. It exists for framework
// storage that holds one executor for the process lifetime, such as the rdb
// session backend; request statements resolve through SQLExecutor instead.
func ConnectionExecutor(ctx context.Context) (sqlbind.SQLExecutor, bool) {
	connection, err := resources(ctx).connection()
	if err != nil {
		return nil, false
	}
	executor := connection.Executor()
	return executor, executor != nil
}

// DBDriver reports the driver scheme of the effective connection, which
// dialect-specific storage needs before it issues SQL.
func DBDriver(ctx context.Context) (string, bool) {
	connection, err := resources(ctx).connection()
	if err != nil {
		return "", false
	}
	return connection.Driver, connection.Driver != ""
}

// SQLExecutor is used by generated .pw.sql context wrappers. It is the one
// seam every generated statement passes through, so query diagnostics attach
// here instead of in generated code or in a wrapping driver.
func SQLExecutor(ctx context.Context) (sqlbind.SQLExecutor, error) {
	current := resources(ctx)
	executor, err := baseSQLExecutor(ctx, current)
	if err != nil {
		return nil, err
	}
	return instrument(current, executor, ReadLogger(ctx)), nil
}

func baseSQLExecutor(ctx context.Context, current *Resources) (sqlbind.SQLExecutor, error) {
	group := current.effectiveGroup()
	if current.TxScope.Active() {
		if current.Connections.Collapsed() || current.TxScope.Group() == group {
			return readOnlyExecutor(current.TxScope.executor(), current.TxScope.ReadOnly()), nil
		}
		// SelectDB named another group inside a transaction. The context
		// executor installed by withScope belongs to that transaction, so it is
		// deliberately skipped: these statements run outside it.
		connection, err := current.connection()
		if err != nil {
			return nil, err
		}
		if !connection.ReadOnly {
			return nil, fmt.Errorf(
				"popcornwave: group %q is writable and cannot be selected inside a transaction on group %q",
				group, current.TxScope.Group())
		}
		return readOnlyExecutor(connection.Executor(), true), nil
	}
	if executor, err := sqlbind.SQLExecutorFromContext(ctx); err == nil {
		return unwrapExecutor(executor), nil
	}
	connection, err := current.connection()
	if err != nil {
		return nil, err
	}
	return readOnlyExecutor(connection.Executor(), connection.ReadOnly), nil
}

// activeScope returns the transaction scope holding an open transaction.
func activeScope(ctx context.Context) *TransactionScope {
	scope := resources(ctx).TxScope
	if scope.Active() {
		return scope
	}
	return nil
}

// withScope installs scope as the request transaction state. Generated SQL
// resolves the scope's executor from the capsule, so no second context node is
// installed beside it: the sqlbind executor key stays an input seam for an
// externally opened transaction, never an output of the framework.
func withScope(ctx context.Context, scope *TransactionScope) context.Context {
	current := derive(ctx)
	current.TxScope = scope
	return WithResources(ctx, current)
}
