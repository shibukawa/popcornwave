// Package pwruntime contains the narrow runtime contract used by generated
// Popcorn Wave code. Handwritten applications should normally import pw.
package pwruntime

import (
	"context"
	"database/sql"
	"errors"
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
	DB            *sql.DB
	// DBDriver is the driver scheme of DSN, used to decide savepoint support.
	DBDriver string
	// TxScope is the active transaction scope, installed by the framework only.
	TxScope *TransactionScope
	// Query enables development query diagnostics. A nil value leaves the
	// resolved executor undecorated.
	Query *QueryDiagnostics
	// Session is the validated session view, installed by session middleware.
	Session *SessionView
	// Authentication is the verified request authentication result, finalized
	// by authentication middleware before handler dispatch.
	Authentication Authentication
}

func WithResources(ctx context.Context, resources Resources) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if resources.Log == nil {
		resources.Log = fallbackBackend()
	}
	return context.WithValue(ctx, contextKey{}, &resources)
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
	current := *resources(ctx)
	current.LogAttributes = mergeAttributes(current.LogAttributes, attributes)
	return WithResources(ctx, current)
}

// WithLogBackend replaces only the emission policy, which is what a test needs
// to capture records without rebuilding every other resource.
func WithLogBackend(ctx context.Context, backend *LogBackend) context.Context {
	current := *resources(ctx)
	current.Log = backend
	return WithResources(ctx, current)
}

func DB(ctx context.Context) (*sql.DB, bool) {
	db := resources(ctx).DB
	return db, db != nil
}

// DBDriver reports the driver scheme of the framework database pool, which
// dialect-specific storage needs before it issues SQL.
func DBDriver(ctx context.Context) (string, bool) {
	driver := resources(ctx).DBDriver
	return driver, driver != ""
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
	if executor := current.TxScope.executor(); executor != nil {
		return executor, nil
	}
	if executor, err := sqlbind.SQLExecutorFromContext(ctx); err == nil {
		return unwrapExecutor(executor), nil
	}
	if db := current.DB; db != nil {
		return db, nil
	}
	return nil, errors.New("popcornwave: database is not available in context")
}

// activeScope returns the transaction scope holding an open transaction.
func activeScope(ctx context.Context) *TransactionScope {
	scope := resources(ctx).TxScope
	if scope.Active() {
		return scope
	}
	return nil
}

// withScope installs scope as the request transaction state and as the
// executor resolved by generated SQL code.
func withScope(ctx context.Context, scope *TransactionScope) context.Context {
	current := *resources(ctx)
	current.TxScope = scope
	ctx = WithResources(ctx, current)
	if executor := scope.executor(); executor != nil {
		ctx = sqlbind.WithSQLExecutor(ctx, executor)
	}
	return ctx
}
