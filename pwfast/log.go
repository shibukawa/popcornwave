package pwfast

import (
	"context"
	"time"

	"github.com/shibukawa/popcornweb/pwruntime"
)

// The logging names pw publishes, under the same spelling, so a rewritten
// handler that logs finds them here.
//
// Logger already crossed; the attribute constructors did not, which made the
// half that crossed unusable: Warn and its siblings take Attribute values and
// nothing here could build one. A handler calling pw.Logger(ctx).Warn(msg,
// pw.Err(err)) — the shape every example writes — was rewritten into a
// reference to a name this package did not declare, and the second build failed
// to compile with no earlier sign. Found 2026-08-11 by the websocket example.
//
// Every declaration is pwruntime's, aliased rather than redeclared, so a record
// written on one transport is the same value on the other.
type (
	Log       = pwruntime.Logger
	Attribute = pwruntime.Attribute
	Level     = pwruntime.Level
)

const (
	LevelTrace = pwruntime.LevelTrace
	LevelDebug = pwruntime.LevelDebug
	LevelInfo  = pwruntime.LevelInfo
	LevelWarn  = pwruntime.LevelWarn
	LevelError = pwruntime.LevelError
	LevelOff   = pwruntime.LevelOff
)

func String(key, value string) Attribute      { return pwruntime.String(key, value) }
func Bool(key string, value bool) Attribute   { return pwruntime.Bool(key, value) }
func Int(key string, value int) Attribute     { return pwruntime.Int(key, value) }
func Int64(key string, value int64) Attribute { return pwruntime.Int64(key, value) }

func Float64(key string, value float64) Attribute {
	return pwruntime.Float64(key, value)
}

func Duration(key string, value time.Duration) Attribute {
	return pwruntime.Duration(key, value)
}

// Err records an error as an attribute rather than as formatted message text,
// so a backend can index it.
func Err(err error) Attribute { return pwruntime.Err(err) }

// WithLogAttributes adds attributes to every record taken from the returned
// context.
func WithLogAttributes(ctx context.Context, attributes ...Attribute) context.Context {
	return pwruntime.WithLogAttributes(ctx, attributes...)
}
