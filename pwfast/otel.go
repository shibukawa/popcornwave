package pwfast

import (
	"net"
	"strconv"
	"strings"

	"github.com/shibukawa/popcornwave/contrib/otel"
	"github.com/shibukawa/popcornwave/contrib/otel/propagation"
	"github.com/shibukawa/popcornwave/contrib/otel/trace"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

type otelConfig struct {
	provider *trace.Provider
	spanName func(*fasthttp.RequestCtx) string
}

// OtelOption configures Otel.
type OtelOption func(*otelConfig)

// WithTracerProvider selects the provider the request spans are created by.
func WithTracerProvider(provider *trace.Provider) OtelOption {
	return func(c *otelConfig) { c.provider = provider }
}

// WithSpanName replaces the default span name.
func WithSpanName(format func(*fasthttp.RequestCtx) string) OtelOption {
	return func(c *otelConfig) {
		if format != nil {
			c.spanName = format
		}
	}
}

// defaultSpanName names a request span "METHOD /path", which is the other
// half's default and the same reasoning: a list where every entry reads GET is
// a list that cannot be read.
func defaultSpanName(r *fasthttp.RequestCtx) string {
	path := string(r.Path())
	if path == "" {
		return string(r.Method())
	}
	return string(r.Method()) + " " + path
}

// Otel extracts W3C Trace Context, opens a server span, and records it on the
// request. With no options it uses trace.DefaultProvider.
//
// The deciding is the shared one and only the reading and recording are this
// transport's, which is the split decision:propagation-header-access describes:
// the version, hex, and tracestate grammar rules live in one function both
// halves call, because a divergence there is a parent silently dropped on one
// transport and accepted on the other, where a divergence in the header calls
// below is a compile error.
//
// The other half derives a context per frame and hands it down. Here the request
// value is the context, so the span is written into it in place and everything
// downstream — SpanContextFromContext, SpanFromContext, a child span — reads it
// unchanged, because the request value answers Value out of the store this
// writes to.
func Otel(options ...OtelOption) Middleware {
	cfg := otelConfig{provider: trace.DefaultProvider(), spanName: defaultSpanName}
	for _, option := range options {
		option(&cfg)
	}
	if cfg.provider == nil {
		cfg.provider = trace.DefaultProvider()
	}
	tracer := cfg.provider.Tracer("github.com/shibukawa/popcornwave/pwfast")
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(r *fasthttp.RequestCtx) {
			parent := trace.ContextWithSpanContext(r, extractedParent(&r.Request.Header))
			_, span := tracer.Start(parent, cfg.spanName(r),
				trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(requestAttributes(r)...))
			trace.StoreSpan(r, span)
			defer func() {
				panicked := recover()
				if panicked != nil {
					span.SetStatus(trace.StatusError, "panic")
					if err, ok := panicked.(error); ok {
						span.RecordError(err)
					}
				}
				status := r.Response.StatusCode()
				if panicked != nil {
					// Recover has not run yet — this frame is inside it — so the
					// status the response carries is whatever the failed handler
					// left, and the 500 the client will see is the true one.
					status = fasthttp.StatusInternalServerError
				}
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

// extractedParent reads the trace context fields off this transport's header.
//
// The strings are built after the presence check rather than before it, so a
// request carrying no traceparent — every request of an untraced caller — costs
// the two lookups and no allocation.
func extractedParent(header *fasthttp.RequestHeader) trace.SpanContext {
	raw := header.PeekAll("traceparent")
	if len(raw) == 0 {
		return trace.SpanContext{}
	}
	// PeekAll rather than Peek, because more than one traceparent names more
	// than one parent and the spec picks no winner; the shared validator refuses
	// the request a parent instead of taking an arbitrary one, and it can only
	// do that if every value reaches it.
	traceparents := make([]string, len(raw))
	for i, value := range raw {
		traceparents[i] = string(value)
	}
	var tracestates []string
	if states := header.PeekAll("tracestate"); len(states) > 0 {
		tracestates = make([]string, len(states))
		for i, value := range states {
			tracestates[i] = string(value)
		}
	}
	sc, _ := propagation.SpanContextFromFields(traceparents, tracestates)
	return sc
}

func requestAttributes(r *fasthttp.RequestCtx) []otel.Attribute {
	// Sized for every append below, because the slice is retained by the span
	// until export and a reallocation here would be one more escaped slice.
	attributes := make([]otel.Attribute, 0, 6)
	attributes = append(attributes,
		otel.String("http.request.method", string(r.Method())),
		otel.String("url.path", string(r.Path())))
	scheme := string(r.URI().Scheme())
	if scheme == "" {
		if r.IsTLS() {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	attributes = append(attributes, otel.String("url.scheme", scheme))
	if query := r.URI().QueryString(); len(query) > 0 {
		attributes = append(attributes, otel.String("url.query", otel.RedactedQuery(string(query))))
	}
	host, port, err := net.SplitHostPort(string(r.Host()))
	if err != nil {
		host = string(r.Host())
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
