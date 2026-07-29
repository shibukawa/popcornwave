package pw

import (
	"context"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// Log is the context-bound logger returned by Logger.
//
// The accessor owns the name Logger, so the type carries the shorter one.
// Handler code normally writes pw.Logger(ctx).Info(...) and never names it.
type Log = pwruntime.Logger

// Attribute is one scalar key-value pair. It is the same type a span attribute
// uses, so one value annotates a record and a span without conversion.
type Attribute = pwruntime.Attribute

// Level is the severity of a record.
type Level = pwruntime.Level

// Severities. Trace sits one step below debug, which slog does not name.
const (
	LevelTrace = pwruntime.LevelTrace
	LevelDebug = pwruntime.LevelDebug
	LevelInfo  = pwruntime.LevelInfo
	LevelWarn  = pwruntime.LevelWarn
	LevelError = pwruntime.LevelError
	LevelOff   = pwruntime.LevelOff
)

// Attribute constructors. Only scalars exist: a record must never fail to
// encode, and a value that needs a structure belongs in its own attributes.
func String(key, value string) Attribute      { return pwruntime.String(key, value) }
func Bool(key string, value bool) Attribute   { return pwruntime.Bool(key, value) }
func Int(key string, value int) Attribute     { return pwruntime.Int(key, value) }
func Int64(key string, value int64) Attribute { return pwruntime.Int64(key, value) }
func Float64(key string, value float64) Attribute {
	return pwruntime.Float64(key, value)
}

// Duration records a duration in milliseconds.
func Duration(key string, value time.Duration) Attribute {
	return pwruntime.Duration(key, value)
}

// Logger returns the logger bound to the request, its stable attributes, and
// the span active on ctx. It never returns something that cannot be called.
//
// Acquire it again inside a child span to correlate records with that span:
//
//	ctx, span := pw.StartSpan(ctx, "load-user")
//	defer span.End()
//	pw.Logger(ctx).Info("loaded", pw.Int("rows", n))
//
// There is no Fatal or Panic. Logging reports what happened; it does not decide
// whether the process keeps running.
func Logger(ctx context.Context) Log { return pwruntime.ReadLogger(ctx) }

// WithLogAttributes adds stable attributes to every record taken from the
// returned context, which is how middleware attaches request-scoped facts.
func WithLogAttributes(ctx context.Context, attributes ...Attribute) context.Context {
	return pwruntime.WithLogAttributes(ctx, attributes...)
}

// Err renders an error as a record attribute. A nil error is safe.
func Err(err error) Attribute { return pwruntime.Err(err) }
