package middlewares

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/shibukawa/popcornwave/contrib/otel"
	"github.com/shibukawa/popcornwave/contrib/otel/propagation"
	"github.com/shibukawa/popcornwave/contrib/otel/trace"
)

type otelConfig struct {
	provider *trace.Provider
	spanName func(*http.Request) string
}
type OtelOption func(*otelConfig)

// defaultSpanName names a request span "METHOD /path".
//
// The method alone is what this used to be, and it makes a trace list unusable:
// every entry reads GET and the one thing distinguishing them is not shown.
//
// Semantic conventions prefer the matched route over the raw path, to bound
// cardinality. The route is not available here: net/http sets it on the request
// the mux hands the handler, which is a copy this middleware never sees, and
// this middleware wraps the mux rather than living inside it. A project that
// needs the route, or that wants the path grouped some other way, replaces this
// through WithSpanName.
func defaultSpanName(r *http.Request) string {
	if r.URL == nil || r.URL.Path == "" {
		return r.Method
	}
	return r.Method + " " + r.URL.Path
}

func WithTracerProvider(provider *trace.Provider) OtelOption {
	return func(c *otelConfig) { c.provider = provider }
}
func WithSpanName(format func(*http.Request) string) OtelOption {
	return func(c *otelConfig) {
		if format != nil {
			c.spanName = format
		}
	}
}

// Otel extracts W3C Trace Context, creates a server span, and installs it in
// the request context. With no options it uses trace.DefaultProvider.
func Otel(options ...OtelOption) Middleware {
	cfg := otelConfig{provider: trace.DefaultProvider(), spanName: defaultSpanName}
	for _, option := range options {
		option(&cfg)
	}
	if cfg.provider == nil {
		cfg.provider = trace.DefaultProvider()
	}
	tracer := cfg.provider.Tracer("github.com/shibukawa/popcornwave/middlewares")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parent := (propagation.TraceContext{}).Extract(r.Context(), r.Header)
			ctx, span := tracer.Start(parent, cfg.spanName(r), trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(requestAttributes(r)...))
			rw, ok := w.(*ResponseTracker)
			if !ok {
				rw = &ResponseTracker{ResponseWriter: w}
			}
			defer func() {
				panicked := recover()
				if panicked != nil {
					rw.status = http.StatusInternalServerError
					span.SetStatus(trace.StatusError, "panic")
					if err, ok := panicked.(error); ok {
						span.RecordError(err)
					}
				}
				span.SetAttributes(otel.Int64("http.response.status_code", int64(rw.Status())))
				if rw.Status() >= 500 {
					span.SetStatus(trace.StatusError, http.StatusText(rw.Status()))
				}
				span.End()
				if panicked != nil {
					panic(panicked)
				}
			}()
			next.ServeHTTP(rw, r.WithContext(ctx))
		})
	}
}

// TraceID returns the current trace ID, or an empty string outside a trace.
func TraceID(ctx context.Context) string { return trace.SpanContextFromContext(ctx).TraceID() }

// SpanID returns the current span ID, or an empty string outside a trace.
func SpanID(ctx context.Context) string { return trace.SpanContextFromContext(ctx).SpanID() }

// StartSpan creates a child span using the tracer selected by Otel.
func StartSpan(ctx context.Context, name string, attributes ...otel.Attribute) (context.Context, *trace.Span) {
	return trace.Start(ctx, name, trace.WithAttributes(attributes...))
}

func requestAttributes(r *http.Request) []otel.Attribute {
	// Sized for every append below, because the slice is retained by the span
	// until export and a reallocation here would be one more escaped slice.
	attributes := make([]otel.Attribute, 0, 6)
	attributes = append(attributes, otel.String("http.request.method", r.Method), otel.String("url.path", r.URL.Path))
	scheme := r.URL.Scheme
	if scheme == "" {
		if r.TLS == nil {
			scheme = "http"
		} else {
			scheme = "https"
		}
	}
	attributes = append(attributes, otel.String("url.scheme", scheme))
	if r.URL.RawQuery != "" {
		attributes = append(attributes, otel.String("url.query", otel.RedactedQuery(r.URL.RawQuery)))
	}
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
		port = ""
	}
	if host != "" {
		attributes = append(attributes, otel.String("server.address", strings.Trim(host, "[]")))
	}
	if number, err := strconv.ParseInt(port, 10, 64); err == nil {
		attributes = append(attributes, otel.Int64("server.port", number))
	}
	return attributes
}
