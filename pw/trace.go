package pw

import (
	"context"
	"net/http"

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

// StartSpan opens a child of the request's active span and returns a context
// carrying it. Outside a trace, or with tracing disabled, it returns a span
// that records nothing and costs nothing to end.
//
//	ctx, span := pw.StartSpan(r, "load-user", pw.String("db.system.name", "sqlite"))
//	defer span.End()
//
// The request root span is created by the framework, so a handler starts only
// the spans that describe its own work. Work below the handler already holds
// the context this returned, and nests through StartSpanContext.
func StartSpan(r *http.Request, name string, attributes ...Attribute) (context.Context, *Span) {
	return trace.Start(r.Context(), name, trace.WithAttributes(attributes...))
}

// StartSpanContext is StartSpan for code below the handler, which nests its
// own span under the one it was given.
func StartSpanContext(ctx context.Context, name string, attributes ...Attribute) (context.Context, *Span) {
	return trace.Start(ctx, name, trace.WithAttributes(attributes...))
}

// StartSpanKind is StartSpan for work that is not internal, such as a call the
// application makes to another service.
func StartSpanKind(r *http.Request, name string, kind SpanKind, attributes ...Attribute) (context.Context, *Span) {
	return trace.Start(r.Context(), name, trace.WithSpanKind(kind), trace.WithAttributes(attributes...))
}

// StartSpanKindContext is StartSpanKind for code below the handler.
func StartSpanKindContext(ctx context.Context, name string, kind SpanKind, attributes ...Attribute) (context.Context, *Span) {
	return trace.Start(ctx, name, trace.WithSpanKind(kind), trace.WithAttributes(attributes...))
}

// TraceID returns the request's trace ID, or an empty string outside a trace.
// It is the value to show a user on an error page so a report can be
// correlated.
func TraceID(r *http.Request) string { return TraceIDContext(r.Context()) }

// TraceIDContext is TraceID for code below the handler.
func TraceIDContext(ctx context.Context) string { return trace.SpanContextFromContext(ctx).TraceID() }

// SpanID returns the span ID active on the request, or an empty string outside
// a trace.
func SpanID(r *http.Request) string { return SpanIDContext(r.Context()) }

// SpanIDContext is SpanID for code below the handler, and reports the span
// that code is running in rather than the request root.
func SpanIDContext(ctx context.Context) string { return trace.SpanContextFromContext(ctx).SpanID() }

// Traced reports whether the request carries a valid span context.
func Traced(r *http.Request) bool { return TracedContext(r.Context()) }

// TracedContext is Traced for code below the handler.
func TracedContext(ctx context.Context) bool { return trace.SpanContextFromContext(ctx).IsValid() }
