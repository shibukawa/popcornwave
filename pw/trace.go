package pw

import (
	"context"

	"github.com/shibukawa/popcornwave/contrib/otel/trace"
)

// Span is one unit of traced work. Always end it, normally with defer.
type Span = trace.Span

// SpanKind classifies a span relative to its peers.
type SpanKind = trace.SpanKind

// Span kinds. The request root span is a server span; a call the application
// makes outward is a client span.
const (
	SpanKindInternal = trace.SpanKindInternal
	SpanKindServer   = trace.SpanKindServer
	SpanKindClient   = trace.SpanKindClient
	SpanKindProducer = trace.SpanKindProducer
	SpanKindConsumer = trace.SpanKindConsumer
)

// Span status codes.
const (
	StatusUnset = trace.StatusUnset
	StatusOK    = trace.StatusOK
	StatusError = trace.StatusError
)

// StartSpan opens a child of the span active on ctx and returns a context
// carrying it. Outside a trace, or with tracing disabled, it returns a span
// that records nothing and costs nothing to end.
//
//	ctx, span := pw.StartSpan(ctx, "load-user", pw.String("db.system.name", "sqlite"))
//	defer span.End()
//
// The request root span is created by the framework, so a handler starts only
// the spans that describe its own work.
func StartSpan(ctx context.Context, name string, attributes ...Attribute) (context.Context, *Span) {
	return trace.Start(ctx, name, trace.WithAttributes(attributes...))
}

// StartSpanKind is StartSpan for work that is not internal, such as a call the
// application makes to another service.
func StartSpanKind(ctx context.Context, name string, kind SpanKind, attributes ...Attribute) (context.Context, *Span) {
	return trace.Start(ctx, name, trace.WithSpanKind(kind), trace.WithAttributes(attributes...))
}

// TraceID returns the current trace ID, or an empty string outside a trace. It
// is the value to show a user on an error page so a report can be correlated.
func TraceID(ctx context.Context) string { return trace.SpanContextFromContext(ctx).TraceID() }

// SpanID returns the current span ID, or an empty string outside a trace.
func SpanID(ctx context.Context) string { return trace.SpanContextFromContext(ctx).SpanID() }

// Traced reports whether ctx carries a valid span context.
func Traced(ctx context.Context) bool { return trace.SpanContextFromContext(ctx).IsValid() }
