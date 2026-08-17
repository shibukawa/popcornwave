// Package otelhttp instruments outgoing HTTP requests with a client span and
// the W3C Trace Context header that continues the trace in the callee.
package otelhttp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/contrib/otel"
	"github.com/shibukawa/popcornwave/contrib/otel/metric"
	"github.com/shibukawa/popcornwave/contrib/otel/propagation"
	"github.com/shibukawa/popcornwave/contrib/otel/trace"
)

const scopeName = "github.com/shibukawa/popcornwave/contrib/otel/otelhttp"

type config struct {
	provider *trace.Provider
	meters   *metric.Provider
	spanName func(*http.Request) string
}

// Option configures a Transport.
type Option func(*config)

// WithTracerProvider selects the provider the client spans are created by.
func WithTracerProvider(provider *trace.Provider) Option {
	return func(c *config) { c.provider = provider }
}

// WithMeterProvider records the http.client instruments on the given provider.
//
// It is opt-in rather than defaulted to the process provider, because a client
// this framework did not build is one whose call sites it cannot vouch for: a
// path-per-object API reached through it would produce one series per object if
// the caller also replaced the span name. A caller that knows its endpoints are
// bounded passes this.
func WithMeterProvider(provider *metric.Provider) Option {
	return func(c *config) { c.meters = provider }
}

// WithSpanName replaces the default span name.
func WithSpanName(format func(*http.Request) string) Option {
	return func(c *config) {
		if format != nil {
			c.spanName = format
		}
	}
}

// defaultSpanName names a client span "METHOD host/path".
//
// The server middleware names its spans "METHOD /path" and this one carries the
// host as well, which is the one difference worth having: a server knows which
// host it is, so repeating it in every span name says nothing, and a client does
// not. Two providers answering "POST /token" are the same entry in a trace list
// until the host is in it.
//
// The path is here for the same reason it is there, and it carries the same
// cardinality caveat: an ID in the path becomes an entry per ID. A caller that
// talks to a path-per-object API replaces this through WithSpanName.
func defaultSpanName(r *http.Request) string {
	if r.URL == nil {
		return r.Method
	}
	return r.Method + " " + r.URL.Host + r.URL.Path
}

// Transport is an http.RoundTripper that opens a client span around the request
// and writes the traceparent naming that span.
//
// The two are one wrapper rather than two because the header names the parent
// the callee adopts. Injecting anywhere that is not the client span names the
// span above it, and the callee's work becomes a sibling of the call that made
// it: the trace still connects, and it describes a call that did not happen that
// way. That is the reasoning decision:outbound-trace-propagation records.
//
// The span ends when RoundTrip returns, so it covers sending the request and
// receiving the response head, and not reading the body. For a request-response
// API the difference is small; for a streamed or large response the span is
// shorter than the transfer, and a caller that needs the transfer timed opens
// its own span around the read.
type Transport struct {
	base     http.RoundTripper
	tracer   *trace.Tracer
	spanName func(*http.Request) string
	// duration and active are the http.client instruments. They are recorded
	// whatever the sampler decided about the span above them, which is the whole
	// reason a metric exists beside a trace.
	duration *metric.Histogram
	active   *metric.UpDownCounter
}

// NewTransport wraps base, or http.DefaultTransport when base is nil.
func NewTransport(base http.RoundTripper, options ...Option) *Transport {
	cfg := config{provider: trace.DefaultProvider(), spanName: defaultSpanName}
	for _, option := range options {
		option(&cfg)
	}
	if cfg.provider == nil {
		cfg.provider = trace.DefaultProvider()
	}
	if base == nil {
		base = http.DefaultTransport
	}
	transport := &Transport{base: base, tracer: cfg.provider.Tracer(scopeName), spanName: cfg.spanName}
	if cfg.meters != nil {
		meter := cfg.meters.Meter(scopeName)
		transport.duration = meter.Histogram("http.client.request.duration", "s",
			"Duration of outbound HTTP requests.", metric.DurationBounds)
		transport.active = meter.UpDownCounter("http.client.active_requests", "{request}",
			"Outbound HTTP requests in flight.")
	}
	return transport
}

// NewClient returns an http.Client whose transport is instrumented.
func NewClient(client *http.Client, options ...Option) *http.Client {
	copied := &http.Client{}
	if client != nil {
		*copied = *client
	}
	copied.Transport = NewTransport(copied.Transport, options...)
	return copied
}

// Unwrap returns the wrapped RoundTripper.
//
// It exists so a consumer that must not be traced can find and remove this
// frame from a client it was handed, which is how the OTLP exporter keeps
// exporting a span from opening a span.
func (t *Transport) Unwrap() http.RoundTripper { return t.base }

func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx, span := t.tracer.Start(r.Context(), t.spanName(r),
		trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(requestAttributes(r)...))
	defer span.End()
	finish := t.measure(r)
	// A RoundTripper may not modify the request it is given, and the header is
	// exactly what this one has to write, so the clone is the contract rather
	// than caution. It also carries ctx, so the span is the one the callee sees.
	outgoing := r.Clone(ctx)
	(propagation.TraceContext{}).Inject(ctx, outgoing.Header)
	response, err := t.base.RoundTrip(outgoing)
	if err != nil {
		message := transportErrorMessage(err)
		finish(0, err)
		span.RecordError(errors.New(message))
		span.SetStatus(trace.StatusError, message)
		return nil, err
	}
	finish(response.StatusCode, nil)
	span.SetAttributes(otel.Int64("http.response.status_code", int64(response.StatusCode)))
	// A 4xx is the caller's own failure here, unlike on the server side where it
	// is the client's, so both error classes fail the client span.
	if response.StatusCode >= 400 {
		span.SetStatus(trace.StatusError, http.StatusText(response.StatusCode))
	}
	return response, nil
}

// transportErrorMessage is the error text with the request URL taken out.
//
// RecordError stores the message verbatim, and a *url.Error prints the URL it
// failed on, query string included. Everywhere else in this package a query
// value is replaced before it can reach a backend; an error message is the one
// path that would have carried the raw one out, and it is the path taken when
// something has already gone wrong and the trace is being read most closely.
//
// The wrapped error keeps the diagnosis — the refused connection, the expired
// certificate, the exceeded deadline — and the URL is already on the span as
// separate attributes that went through the redaction.
func transportErrorMessage(err error) string {
	var wrapped *url.Error
	if errors.As(err, &wrapped) && wrapped.Err != nil {
		return wrapped.Err.Error()
	}
	return err.Error()
}

// measure counts one outbound request and returns the call that closes it.
//
// The attribute set names the callee and never the path: a client talks to
// addresses a caller chose, and a path-per-object API would otherwise be one
// series per object. The span beside it carries the path, where one call is
// being read rather than all of them.
func (t *Transport) measure(r *http.Request) func(status int, err error) {
	if t.duration == nil {
		return func(int, error) {}
	}
	started := time.Now()
	host, port := "", ""
	if r.URL != nil {
		host = r.URL.Hostname()
		port = r.URL.Port()
	}
	base := []otel.Attribute{
		otel.String("http.request.method", r.Method),
		otel.String("server.address", host),
	}
	if port != "" {
		if number, convErr := strconv.ParseInt(port, 10, 64); convErr == nil {
			base = append(base, otel.Int64("server.port", number))
		}
	}
	t.active.Add(r.Context(), 1, base...)
	return func(status int, err error) {
		t.active.Add(r.Context(), -1, base...)
		attributes := append([]otel.Attribute(nil), base...)
		if status > 0 {
			attributes = append(attributes, otel.Int64("http.response.status_code", int64(status)))
		}
		if err != nil {
			attributes = append(attributes, otel.String("error.type", errorType(err)))
		}
		t.duration.Record(r.Context(), time.Since(started).Seconds(), attributes...)
	}
}

// errorType is the closed-set error.type value: the type name, never the
// message, which carries an address and a path a series must not be split by.
func errorType(err error) string {
	name := strings.TrimPrefix(fmt.Sprintf("%T", err), "*")
	if name == "" {
		return "_OTHER"
	}
	return name
}

func requestAttributes(r *http.Request) []otel.Attribute {
	// Sized for every append below, because the slice is retained by the span
	// until export and a reallocation here would be one more escaped slice.
	attributes := make([]otel.Attribute, 0, 6)
	attributes = append(attributes, otel.String("http.request.method", r.Method))
	if r.URL == nil {
		return attributes
	}
	attributes = append(attributes, otel.String("url.path", r.URL.Path))
	if r.URL.Scheme != "" {
		attributes = append(attributes, otel.String("url.scheme", r.URL.Scheme))
	}
	if r.URL.RawQuery != "" {
		attributes = append(attributes, otel.String("url.query", otel.RedactedQuery(r.URL.RawQuery)))
	}
	host, port, err := net.SplitHostPort(r.URL.Host)
	if err != nil {
		host = r.URL.Host
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
