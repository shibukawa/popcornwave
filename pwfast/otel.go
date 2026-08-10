package pwfast

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/shibukawa/popcornwave/contrib/otel"
	"github.com/shibukawa/popcornwave/contrib/otel/propagation"
	"github.com/shibukawa/popcornwave/contrib/otel/trace"
	"github.com/shibukawa/popcornwave/internal/spanattr"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

type otelConfig struct {
	provider *trace.Provider
	spanName func(*fasthttp.RequestCtx) string
}

// OtelOption configures Otel.
type OtelOption func(*otelConfig)

// WithTracerProvider replaces the provider spans are created from.
func WithTracerProvider(provider *trace.Provider) OtelOption {
	return func(c *otelConfig) { c.provider = provider }
}

// WithSpanName replaces how a request span is named.
func WithSpanName(format func(*fasthttp.RequestCtx) string) OtelOption {
	return func(c *otelConfig) {
		if format != nil {
			c.spanName = format
		}
	}
}

// defaultSpanName names a request span "METHOD /path", which is what the other
// transport names it. The method alone makes a trace list unusable: every entry
// reads GET and the one thing distinguishing them is not shown.
func defaultSpanName(r *fasthttp.RequestCtx) string {
	path := string(r.Path())
	if path == "" {
		return string(r.Method())
	}
	return string(r.Method()) + " " + path
}

// Otel extracts W3C Trace Context, creates a server span, and installs it on
// the request. With no options it uses trace.DefaultProvider.
//
// The span is opened outside every positioned frame, so a request root span
// covers the whole chain and every record taken inside it correlates. That is
// why its slot is the smallest one.
func Otel(options ...OtelOption) Middleware {
	config := otelConfig{provider: trace.DefaultProvider(), spanName: defaultSpanName}
	for _, option := range options {
		option(&config)
	}
	if config.provider == nil {
		config.provider = trace.DefaultProvider()
	}
	tracer := config.provider.Tracer("github.com/shibukawa/popcornwave/pwfast")
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(r *fasthttp.RequestCtx) {
			parent := (propagation.TraceContext{}).Extract(r, traceHeader(r))
			ctx, span := tracer.Start(parent, config.spanName(r),
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(requestAttributes(r)...))
			// The derived context carries the span, and this transport cannot
			// hand a derived context to the next handler — so the span is
			// written onto the request value, where every reader looks.
			trace.StoreContext(r, ctx)
			defer func() {
				panicked := recover()
				if panicked != nil {
					// The status is recorded before re-panicking, because the
					// recover frame above this one will answer 500 and the span
					// would otherwise close saying 200.
					r.SetStatusCode(fasthttp.StatusInternalServerError)
					span.SetStatus(trace.StatusError, "panic")
					if err, ok := panicked.(error); ok {
						span.RecordError(err)
					}
				}
				status := r.Response.StatusCode()
				span.SetAttributes(otel.Int64("http.response.status_code", int64(status)))
				if status >= 500 {
					span.SetStatus(trace.StatusError, fasthttp.StatusMessage(status))
				}
				span.End()
				if panicked != nil {
					panic(panicked)
				}
			}()
			next(r)
		}
	}
}

// traceHeader lifts the two headers W3C Trace Context propagation reads into
// the map the extractor takes.
//
// Only those two are copied. The extractor takes an http.Header, which is a
// plain map rather than a transport, and copying the whole header set to hand
// over two entries would allocate a map per request for nothing.
func traceHeader(r *fasthttp.RequestCtx) http.Header {
	header := make(http.Header, 2)
	for _, name := range []string{"traceparent", "tracestate"} {
		if value := r.Request.Header.Peek(name); len(value) > 0 {
			header.Set(name, string(value))
		}
	}
	return header
}

// requestAttributes describes the request on its span, following the same
// semantic conventions the other transport follows.
func requestAttributes(r *fasthttp.RequestCtx) []otel.Attribute {
	// Sized for every append below, because the slice is retained by the span
	// until export and a reallocation here would be one more escaped slice.
	attributes := make([]otel.Attribute, 0, 6)
	attributes = append(attributes,
		otel.String("http.request.method", string(r.Method())),
		otel.String("url.path", string(r.Path())))
	scheme := "http"
	if r.IsTLS() {
		scheme = "https"
	}
	attributes = append(attributes, otel.String("url.scheme", scheme))
	if query := string(r.URI().QueryString()); query != "" {
		// The values are dropped through the shared redaction. A trace backend
		// is retained longer and read more widely than the application
		// database, and a query string is where a password-reset token, an
		// OAuth code and a presigned signature all travel.
		attributes = append(attributes, otel.String("url.query", spanattr.RedactQuery(query)))
	}
	host, port, err := net.SplitHostPort(string(r.Host()))
	if err != nil {
		host, port = string(r.Host()), ""
	}
	if host != "" {
		attributes = append(attributes, otel.String("server.address", strings.Trim(host, "[]")))
	}
	if number, err := strconv.ParseInt(port, 10, 64); err == nil {
		attributes = append(attributes, otel.Int64("server.port", number))
	}
	return attributes
}
