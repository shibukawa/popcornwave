package otelhttp

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/shibukawa/popcornweb/contrib/otel/propagation"
	"github.com/shibukawa/popcornweb/contrib/otel/trace"
)

type spanCollector struct{ spans []trace.SpanData }

func (c *spanCollector) OnEnd(span trace.SpanData)    { c.spans = append(c.spans, span) }
func (*spanCollector) Shutdown(context.Context) error { return nil }

// recorder captures the request the wrapped transport was actually given.
type recorder struct {
	got    *http.Request
	status int
	err    error
}

func (rt *recorder) RoundTrip(r *http.Request) (*http.Response, error) {
	rt.got = r
	if rt.err != nil {
		return nil, rt.err
	}
	status := rt.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{StatusCode: status, Body: http.NoBody, Request: r}, nil
}

func attribute(span trace.SpanData, key string) (string, bool) {
	for _, a := range span.Attributes {
		if a.Key == key {
			value, _ := a.Value.AsString()
			return value, true
		}
	}
	return "", false
}

func request(t *testing.T, ctx context.Context, target string) *http.Request {
	t.Helper()
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// The header must name the client span rather than the span above it. If it
// named the caller, the callee's work would come back as a sibling of the call
// that produced it, and the trace would describe a request nobody made.
func TestInjectedParentIsTheClientSpan(t *testing.T) {
	collector := &spanCollector{}
	provider := trace.NewProvider(collector)
	base := &recorder{}
	transport := NewTransport(base, WithTracerProvider(provider))

	server, serverSpan := provider.Tracer("server").Start(context.Background(), "GET /page",
		trace.WithSpanKind(trace.SpanKindServer))
	response, err := transport.RoundTrip(request(t, server, "https://api.example.com/v1/users"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	serverSpan.End()

	if len(collector.spans) != 2 {
		t.Fatalf("completed spans = %d, want the client span and the server span", len(collector.spans))
	}
	client := collector.spans[0]
	if client.Kind != trace.SpanKindClient {
		t.Fatalf("first completed span kind = %v, want the client span", client.Kind)
	}

	sc, ok := propagation.SpanContextFromFields(base.got.Header.Values("traceparent"), nil)
	if !ok {
		t.Fatal("no traceparent reached the wrapped transport")
	}
	if sc.SpanID() != client.SpanContext.SpanID() {
		t.Errorf("traceparent names span %q, want the client span %q", sc.SpanID(), client.SpanContext.SpanID())
	}
	if sc.SpanID() == serverSpan.SpanContext().SpanID() {
		t.Error("traceparent names the server span, so the callee becomes a sibling of this call")
	}
	if sc.TraceID() != serverSpan.SpanContext().TraceID() {
		t.Errorf("traceparent trace %q left the trace it belongs to", sc.TraceID())
	}
}

// A RoundTripper may not modify the request it is handed.
func TestRoundTripLeavesTheCallersRequestAlone(t *testing.T) {
	base := &recorder{}
	transport := NewTransport(base, WithTracerProvider(trace.NewProvider(&spanCollector{})))

	ctx, span := trace.NewProvider(&spanCollector{}).Tracer("caller").Start(context.Background(), "caller")
	defer span.End()
	original := request(t, ctx, "https://api.example.com/v1/users")
	response, err := transport.RoundTrip(original)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()

	if got := original.Header.Get("traceparent"); got != "" {
		t.Errorf("the caller's own request gained a traceparent: %q", got)
	}
	if base.got == original {
		t.Error("the wrapped transport was handed the caller's request rather than a clone")
	}
	if base.got.Header.Get("traceparent") == "" {
		t.Error("the clone carries no traceparent")
	}
}

// An untraced process sends no header, with nothing to configure.
func TestNoSpanContextSendsNoHeader(t *testing.T) {
	base := &recorder{}
	transport := NewTransport(base, WithTracerProvider(trace.NewProvider(nil)))
	// A background context has no span, so the client span this opens is a root
	// and the header it writes is that root, which is a trace of one call.
	response, err := transport.RoundTrip(request(t, context.Background(), "https://api.example.com/x"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if base.got.Header.Get("traceparent") == "" {
		t.Error("a root client span still starts a trace the callee can join")
	}
}

// The query string is redacted on the way to the backend, exactly as it is on
// the server span.
func TestQueryValuesDoNotReachTheClientSpan(t *testing.T) {
	collector := &spanCollector{}
	transport := NewTransport(&recorder{}, WithTracerProvider(trace.NewProvider(collector)))
	response, err := transport.RoundTrip(request(t, context.Background(),
		"https://api.example.com/token?code=abc123&state=xyz"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()

	query, ok := attribute(collector.spans[0], "url.query")
	if !ok {
		t.Fatal("no url.query attribute")
	}
	if want := "code=REDACTED&state=REDACTED"; query != want {
		t.Errorf("url.query = %q, want %q", query, want)
	}
	if host, _ := attribute(collector.spans[0], "server.address"); host != "api.example.com" {
		t.Errorf("server.address = %q", host)
	}
}

// A failed request records why it failed and not the URL it failed on, which a
// *url.Error prints in full, query string included.
func TestErrorMessageDropsTheURL(t *testing.T) {
	collector := &spanCollector{}
	failure := &url.Error{
		Op:  "Get",
		URL: "https://api.example.com/token?code=abc123",
		Err: errors.New("dial tcp 10.0.0.1:443: connect: connection refused"),
	}
	transport := NewTransport(&recorder{err: failure}, WithTracerProvider(trace.NewProvider(collector)))
	if _, err := transport.RoundTrip(request(t, context.Background(), failure.URL)); err == nil {
		t.Fatal("the caller was not told the request failed")
	}

	span := collector.spans[0]
	if span.Status != trace.StatusError {
		t.Errorf("status = %v, want error", span.Status)
	}
	if strings.Contains(span.StatusDescription, "abc123") {
		t.Errorf("the status description leaked the query: %q", span.StatusDescription)
	}
	if !strings.Contains(span.StatusDescription, "connection refused") {
		t.Errorf("the status description lost the diagnosis: %q", span.StatusDescription)
	}
	for _, event := range span.Events {
		for _, a := range event.Attributes {
			if value, _ := a.Value.AsString(); strings.Contains(value, "abc123") {
				t.Errorf("the recorded exception leaked the query: %q", value)
			}
		}
	}
}

// A client span fails on 4xx as well as 5xx, unlike the server span, because
// here the failed request is this process's own.
func TestClientErrorFailsTheSpan(t *testing.T) {
	collector := &spanCollector{}
	transport := NewTransport(&recorder{status: http.StatusNotFound},
		WithTracerProvider(trace.NewProvider(collector)))
	response, err := transport.RoundTrip(request(t, context.Background(), "https://api.example.com/missing"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()

	if collector.spans[0].Status != trace.StatusError {
		t.Errorf("status = %v, want error for a 404 the caller asked for", collector.spans[0].Status)
	}
	var code int64
	for _, a := range collector.spans[0].Attributes {
		if a.Key == "http.response.status_code" {
			code, _ = a.Value.AsInt64()
		}
	}
	if code != http.StatusNotFound {
		t.Errorf("http.response.status_code = %d", code)
	}
}

func TestNewClientInstrumentsWithoutMutatingTheOriginal(t *testing.T) {
	base := &recorder{}
	original := &http.Client{Transport: base}
	traced := NewClient(original, WithTracerProvider(trace.NewProvider(&spanCollector{})))

	if original.Transport != http.RoundTripper(base) {
		t.Error("NewClient replaced the transport of the client it was given")
	}
	wrapper, ok := traced.Transport.(*Transport)
	if !ok {
		t.Fatalf("traced transport = %T", traced.Transport)
	}
	if wrapper.Unwrap() != http.RoundTripper(base) {
		t.Error("Unwrap did not return the wrapped transport")
	}
}
