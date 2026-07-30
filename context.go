package petitweb

import (
	"context"

	"github.com/shibukawa/popcornwave/pwruntime"
)

type contextKey struct{}

type requestValues struct {
	requestID string
	logger    pwruntime.Logger
}

func withRequestValues(ctx context.Context, values requestValues) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKey{}, &values)
}

func readRequestValues(ctx context.Context) (*requestValues, bool) {
	if ctx == nil {
		return nil, false
	}
	values, ok := ctx.Value(contextKey{}).(*requestValues)
	return values, ok && values != nil
}

// ReadRequestID returns the validated request correlation ID.
func ReadRequestID(ctx context.Context) (string, bool) {
	values, ok := readRequestValues(ctx)
	if !ok || values.requestID == "" {
		return "", false
	}
	return values.requestID, true
}

// Logger is the context-bound logger returned by ReadLogger.
type Logger = pwruntime.Logger

// Attribute is one scalar key-value pair on a record.
type Attribute = pwruntime.Attribute

// Severities.
const (
	LevelTrace = pwruntime.LevelTrace
	LevelDebug = pwruntime.LevelDebug
	LevelInfo  = pwruntime.LevelInfo
	LevelWarn  = pwruntime.LevelWarn
	LevelError = pwruntime.LevelError
)

// Attribute constructors.
func String(key, value string) Attribute          { return pwruntime.String(key, value) }
func Bool(key string, value bool) Attribute       { return pwruntime.Bool(key, value) }
func Int(key string, value int) Attribute         { return pwruntime.Int(key, value) }
func Int64(key string, value int64) Attribute     { return pwruntime.Int64(key, value) }
func Float64(key string, value float64) Attribute { return pwruntime.Float64(key, value) }

// Err renders an error as a record attribute. A nil error is safe.
func Err(err error) Attribute { return pwruntime.Err(err) }

// ReadLogger always returns a usable request-aware logger. A request that never
// passed through RequestID still gets the logger installed on the context, and
// a context with nothing installed still gets one that can be called.
func ReadLogger(ctx context.Context) Logger {
	if values, ok := readRequestValues(ctx); ok {
		return values.logger
	}
	return pwruntime.ReadLogger(ctx)
}
