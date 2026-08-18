package pwruntime

import (
	"context"
	"log/slog"

	"github.com/shibukawa/popcornweb/contrib/otel"
	otellog "github.com/shibukawa/popcornweb/contrib/otel/log"
	"github.com/shibukawa/popcornweb/contrib/otel/trace"
)

// Reserved record fields. A user attribute may not take one of these names,
// because a consumer reading a stream has to be able to trust them.
const (
	FieldTimestamp   = "timestamp"
	FieldSeverity    = "severity"
	FieldMessage     = "message"
	FieldServiceName = "service_name"
	FieldTraceID     = "trace_id"
	FieldSpanID      = "span_id"
	FieldTraceFlags  = "trace_flags"
)

func reserved(key string) bool {
	switch key {
	case FieldTimestamp, FieldSeverity, FieldMessage, FieldServiceName, FieldTraceID, FieldSpanID, FieldTraceFlags:
		return true
	default:
		return false
	}
}

// SlogSink writes records through a slog.Handler.
//
// slog owns the encoding rather than a private formatter because it already
// ships the JSON and text forms this framework needs, an application can
// substitute any handler it already uses, and the result stays readable on
// TinyGo, where slog works apart from source locations.
type SlogSink struct{ handler slog.Handler }

// NewSlogSink returns a sink writing to handler. A nil handler yields a nil
// Sink, which NewLogBackend drops.
//
// The return type is the interface rather than *SlogSink on purpose: a typed
// nil pointer stored in an interface is not nil, so returning the concrete type
// would make "no destination configured" silently look like a destination.
func NewSlogSink(handler slog.Handler) Sink {
	if handler == nil {
		return nil
	}
	return &SlogSink{handler: handler}
}

func (sink *SlogSink) Emit(ctx context.Context, record Record) {
	if sink == nil || sink.handler == nil {
		return
	}
	level := slog.Level(record.Level)
	if !sink.handler.Enabled(ctx, level) {
		return
	}
	entry := slog.NewRecord(record.Time, level, record.Message, 0)
	if record.TraceID != "" {
		entry.AddAttrs(
			slog.String(FieldTraceID, record.TraceID),
			slog.String(FieldSpanID, record.SpanID),
			slog.Int(FieldTraceFlags, int(record.TraceFlags)),
		)
	}
	for _, attribute := range record.Attributes {
		if reserved(attribute.Key) {
			continue
		}
		entry.AddAttrs(slogAttr(attribute))
	}
	_ = sink.handler.Handle(ctx, entry)
}

// slogAttr keeps each scalar typed rather than pre-rendering it to a string, so
// a JSON consumer still sees a number as a number.
func slogAttr(attribute Attribute) slog.Attr {
	switch attribute.Value.Kind() {
	case otel.BoolKind:
		value, _ := attribute.Value.AsBool()
		return slog.Bool(attribute.Key, value)
	case otel.Int64Kind:
		value, _ := attribute.Value.AsInt64()
		return slog.Int64(attribute.Key, value)
	case otel.Float64Kind:
		value, _ := attribute.Value.AsFloat64()
		return slog.Float64(attribute.Key, value)
	default:
		value, _ := attribute.Value.AsString()
		return slog.String(attribute.Key, value)
	}
}

// OtelSink emits records through an OpenTelemetry log provider.
type OtelSink struct{ logger *otellog.Logger }

// NewOtelSink returns a sink emitting through logger. A nil logger yields a nil
// Sink for the same reason NewSlogSink does.
func NewOtelSink(logger *otellog.Logger) Sink {
	if logger == nil {
		return nil
	}
	return &OtelSink{logger: logger}
}

func (sink *OtelSink) Emit(ctx context.Context, record Record) {
	if sink == nil || sink.logger == nil {
		return
	}
	// The common record carries no reserved key, and then the slice is shared
	// as is — attribute slices are immutable once handed over — rather than
	// copied per record. The copy happens only at the first reserved key.
	attributes := record.Attributes
	for index, attribute := range attributes {
		if !reserved(attribute.Key) {
			continue
		}
		kept := make([]Attribute, index, len(attributes)-1)
		copy(kept, attributes[:index])
		for _, candidate := range attributes[index+1:] {
			if !reserved(candidate.Key) {
				kept = append(kept, candidate)
			}
		}
		attributes = kept
		break
	}
	sink.logger.Emit(correlatedContext(ctx, record), otellog.Record{
		Timestamp:    record.Time,
		Severity:     otelSeverity(record.Level),
		SeverityText: record.Level.String(),
		Body:         record.Message,
		Attributes:   attributes,
	})
}

// correlatedContext restores the span the record was bound to.
//
// The provider reads correlation from the context, but a record carries the
// span captured when its logger was acquired, which is not necessarily the span
// active where the record is emitted: Logger.Info takes no context at all.
// Putting the captured span back is what keeps a record and its span together
// in the exported payload.
func correlatedContext(ctx context.Context, record Record) context.Context {
	if record.TraceID == "" {
		return ctx
	}
	if active := trace.SpanContextFromContext(ctx); active.IsValid() && active.SpanID() == record.SpanID {
		return ctx
	}
	spanContext, err := trace.NewSpanContext(record.TraceID, record.SpanID, record.TraceFlags, "", false)
	if err != nil {
		return ctx
	}
	return trace.ContextWithSpanContext(ctx, spanContext)
}

// otelSeverity maps a framework level onto the OpenTelemetry severity numbers.
// Fatal has no source here, because the logger has no method that ends a
// process.
func otelSeverity(level Level) otellog.SeverityNumber {
	switch {
	case level <= LevelTrace:
		return otellog.SeverityTrace
	case level <= LevelDebug:
		return otellog.SeverityDebug
	case level <= LevelInfo:
		return otellog.SeverityInfo
	case level <= LevelWarn:
		return otellog.SeverityWarn
	default:
		return otellog.SeverityError
	}
}
