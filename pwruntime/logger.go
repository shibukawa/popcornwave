package pwruntime

import (
	"context"
	"log/slog"
	"time"

	"github.com/shibukawa/popcornwave/contrib/otel"
	"github.com/shibukawa/popcornwave/contrib/otel/trace"
)

// Level is the severity of a log record.
//
// The numbers are slog's, so a Level converts to a slog.Level without a table
// and a handler written against slog filters these records correctly. Trace is
// the one severity slog does not name; it sits one step below debug.
type Level int8

const (
	LevelTrace Level = Level(slog.LevelDebug) - 4
	LevelDebug Level = Level(slog.LevelDebug)
	LevelInfo  Level = Level(slog.LevelInfo)
	LevelWarn  Level = Level(slog.LevelWarn)
	LevelError Level = Level(slog.LevelError)
	// LevelOff is above every severity, so nothing passes the filter.
	LevelOff Level = 127
)

// String renders the severity as the lowercase token used by configuration.
func (level Level) String() string {
	switch {
	case level <= LevelTrace:
		return "trace"
	case level <= LevelDebug:
		return "debug"
	case level <= LevelInfo:
		return "info"
	case level <= LevelWarn:
		return "warn"
	case level < LevelOff:
		return "error"
	default:
		return "off"
	}
}

// Attribute is one scalar key-value pair on a record.
//
// It is the same type contrib/otel puts on a span, so a value that annotates a
// span annotates a record without conversion, and neither surface can carry a
// structure whose encoding could fail while a request is being served.
type Attribute = otel.Attribute

// Attribute constructors, re-exported so application code needs no contrib
// import to write a record.
func String(key, value string) Attribute          { return otel.String(key, value) }
func Bool(key string, value bool) Attribute       { return otel.Bool(key, value) }
func Int64(key string, value int64) Attribute     { return otel.Int64(key, value) }
func Float64(key string, value float64) Attribute { return otel.Float64(key, value) }

// Int is the convenience form of Int64 for the counts and sizes framework code
// mostly logs.
func Int(key string, value int) Attribute { return otel.Int64(key, int64(value)) }

// Duration records a duration in milliseconds, which is the unit both the
// OpenTelemetry log model and a human reading a terminal expect.
func Duration(key string, value time.Duration) Attribute {
	return otel.Float64(key, float64(value)/float64(time.Millisecond))
}

// Record is one finished log record handed to a Sink. Correlation is resolved
// before the record is built, so a Sink never inspects the context for it.
type Record struct {
	Time       time.Time
	Level      Level
	Message    string
	Attributes []Attribute
	TraceID    string
	SpanID     string
	TraceFlags byte
}

// Sink receives records that passed the severity filter. Emitting must not
// panic and must not block the request goroutine for longer than a bounded
// enqueue: a logger that can stop a handler is worse than a missing record.
type Sink interface {
	Emit(ctx context.Context, record Record)
}

// LogBackend is the process-wide emission policy: one severity floor and the
// set of destinations a record reaches.
//
// More than one sink is how a development run keeps its terminal stream while
// also feeding a collector. A production run configured for OTLP normally holds
// one sink, because duplicate delivery is a cost nobody asked for.
type LogBackend struct {
	minimum Level
	sinks   []Sink
}

// NewLogBackend returns a backend that drops records below minimum and hands
// the rest to every sink in order. A backend with no sink is valid and silent.
func NewLogBackend(minimum Level, sinks ...Sink) *LogBackend {
	live := make([]Sink, 0, len(sinks))
	for _, sink := range sinks {
		if sink != nil {
			live = append(live, sink)
		}
	}
	return &LogBackend{minimum: minimum, sinks: live}
}

// Sinks reports how many destinations a record reaches, which is what
// distinguishes exclusive routing from the development fan-out.
func (backend *LogBackend) Sinks() int {
	if backend == nil {
		return 0
	}
	return len(backend.sinks)
}

// Enabled reports whether a record at level would reach any destination.
func (backend *LogBackend) Enabled(level Level) bool { return backend.enabled(level) }

// Minimum reports the configured severity floor.
func (backend *LogBackend) Minimum() Level {
	if backend == nil {
		return LevelInfo
	}
	return backend.minimum
}

func (backend *LogBackend) enabled(level Level) bool {
	if backend == nil || len(backend.sinks) == 0 {
		return false
	}
	return level >= backend.minimum && backend.minimum != LevelOff
}

func (backend *LogBackend) emit(ctx context.Context, record Record) {
	for _, sink := range backend.sinks {
		sink.Emit(ctx, record)
	}
}

// Logger is the context-bound logger of the framework logging API.
//
// It is a value: With returns a new one and never mutates the receiver, so a
// logger captured by a handler cannot be changed underneath it. The zero value
// is usable and discards everything, which is what keeps every accessor free to
// return a logger rather than nil.
type Logger struct {
	backend    *LogBackend
	attributes []Attribute
	traceID    string
	spanID     string
	traceFlags byte
}

// NewLogger binds backend and the trace correlation of ctx into a logger.
func NewLogger(ctx context.Context, backend *LogBackend, attributes ...Attribute) Logger {
	logger := Logger{backend: backend, attributes: attributes}
	return logger.withTraceOf(ctx)
}

// withTraceOf captures the span active on ctx. Correlation is read once, at
// acquisition, because a logger held across a child span must keep pointing at
// the span it was taken from.
func (logger Logger) withTraceOf(ctx context.Context) Logger {
	span := trace.SpanContextFromContext(ctx)
	if !span.IsValid() {
		return logger
	}
	logger.traceID, logger.spanID, logger.traceFlags = span.TraceID(), span.SpanID(), span.TraceFlags()
	return logger
}

// Enabled reports whether a record at level would reach any destination. Call
// it before assembling attributes that cost something to produce.
func (logger Logger) Enabled(level Level) bool { return logger.backend.enabled(level) }

// With returns a logger carrying attributes in addition to its own. A later
// duplicate key replaces an earlier one when the record is built.
func (logger Logger) With(attributes ...Attribute) Logger {
	if len(attributes) == 0 {
		return logger
	}
	merged := make([]Attribute, 0, len(logger.attributes)+len(attributes))
	merged = append(merged, logger.attributes...)
	merged = append(merged, attributes...)
	logger.attributes = merged
	return logger
}

// TraceID reports the trace this logger is correlated with, if any.
func (logger Logger) TraceID() string { return logger.traceID }

// SpanID reports the span this logger is correlated with, if any.
func (logger Logger) SpanID() string { return logger.spanID }

func (logger Logger) Trace(message string, attributes ...Attribute) {
	logger.log(context.Background(), LevelTrace, message, attributes)
}
func (logger Logger) Debug(message string, attributes ...Attribute) {
	logger.log(context.Background(), LevelDebug, message, attributes)
}
func (logger Logger) Info(message string, attributes ...Attribute) {
	logger.log(context.Background(), LevelInfo, message, attributes)
}
func (logger Logger) Warn(message string, attributes ...Attribute) {
	logger.log(context.Background(), LevelWarn, message, attributes)
}
func (logger Logger) Error(message string, attributes ...Attribute) {
	logger.log(context.Background(), LevelError, message, attributes)
}

// Log emits at an explicit level with the caller's context, which a sink may
// use for cancellation. There is deliberately no Fatal or Panic: logging must
// not decide whether the process lives.
func (logger Logger) Log(ctx context.Context, level Level, message string, attributes ...Attribute) {
	logger.log(ctx, level, message, attributes)
}

func (logger Logger) log(ctx context.Context, level Level, message string, attributes []Attribute) {
	if !logger.backend.enabled(level) {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	logger.backend.emit(ctx, Record{
		Time:       time.Now(),
		Level:      level,
		Message:    message,
		Attributes: mergeAttributes(logger.attributes, attributes),
		TraceID:    logger.traceID,
		SpanID:     logger.spanID,
		TraceFlags: logger.traceFlags,
	})
}

// mergeAttributes appends call attributes to bound ones and resolves duplicate
// keys deterministically: the later value wins, wherever it came from.
func mergeAttributes(bound, call []Attribute) []Attribute {
	if len(bound) == 0 && len(call) == 0 {
		return nil
	}
	merged := make([]Attribute, 0, len(bound)+len(call))
	merged = append(merged, bound...)
	for _, attribute := range call {
		replaced := false
		for i := range merged {
			if merged[i].Key == attribute.Key {
				merged[i] = attribute
				replaced = true
				break
			}
		}
		if !replaced {
			merged = append(merged, attribute)
		}
	}
	return merged
}

// Err renders an error as a record attribute. A nil error yields an empty
// value instead of panicking, because a log site is the last place that should
// introduce a crash.
func Err(err error) Attribute {
	if err == nil {
		return otel.String("error", "")
	}
	return otel.String("error", err.Error())
}
