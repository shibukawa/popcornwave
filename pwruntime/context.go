// Package pwruntime contains the narrow runtime contract used by generated
// Popcorn Wave code. Handwritten applications should normally import pw.
package pwruntime

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"reflect"

	"github.com/shibukawa/tinybind-go/sqlbind"
)

type contextKey struct{}

// Resources is the immutable process/request state installed by pw.
type Resources struct {
	Configs map[reflect.Type]any
	Logger  *slog.Logger
	DB      *sql.DB
}

func WithResources(ctx context.Context, resources Resources) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if resources.Logger == nil {
		resources.Logger = slog.Default()
	}
	return context.WithValue(ctx, contextKey{}, &resources)
}

func resources(ctx context.Context) *Resources {
	if ctx != nil {
		if value, ok := ctx.Value(contextKey{}).(*Resources); ok && value != nil {
			return value
		}
	}
	return &Resources{Logger: slog.Default()}
}

func Config[T any](ctx context.Context) (T, bool) {
	var zero T
	value, ok := resources(ctx).Configs[reflect.TypeFor[T]()]
	if !ok {
		return zero, false
	}
	typed, ok := value.(T)
	return typed, ok
}

func Logger(ctx context.Context) *slog.Logger {
	logger := resources(ctx).Logger
	if logger == nil {
		return slog.Default()
	}
	return logger
}

// WithLogger replaces only the request logger while preserving other runtime
// resources already installed on ctx.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	current := *resources(ctx)
	current.Logger = logger
	return WithResources(ctx, current)
}

func DB(ctx context.Context) (*sql.DB, bool) {
	db := resources(ctx).DB
	return db, db != nil
}

// SQLExecutor is used by generated .pw.sql context wrappers.
func SQLExecutor(ctx context.Context) (sqlbind.SQLExecutor, error) {
	if executor, err := sqlbind.SQLExecutorFromContext(ctx); err == nil {
		return executor, nil
	}
	if db, ok := DB(ctx); ok {
		return db, nil
	}
	return nil, errors.New("popcornwave: database is not available in context")
}
