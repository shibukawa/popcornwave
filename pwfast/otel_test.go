package pwfast

import (
	"context"
	"testing"

	"github.com/shibukawa/popcornwave/contrib/otel/trace"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

type spanCollector struct{ spans []trace.SpanData }

func (c *spanCollector) OnEnd(span trace.SpanData)    { c.spans = append(c.spans, span) }
func (*spanCollector) Shutdown(context.Context) error { return nil }

const remoteParent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func attributeOf(span trace.SpanData, key string) string {
	for _, a := range span.Attributes {
		if a.Key == key {
			value, _ := a.Value.AsString()
			return value
		}
	}
	return ""
}

// The point of the whole port: a request arriving on this transport with a
// traceparent continues the caller's trace instead of starting a new one.
func TestOtelContinuesTheCallersTrace(t *testing.T) {
	collector := &spanCollector{}
	middleware := Otel(WithTracerProvider(trace.NewProvider(collector)))
	serveRaw(t, Chain(func(*fasthttp.RequestCtx) {}, middleware), "/orders",
		"Traceparent: "+remoteParent+"\r\nTracestate: vendor=value\r\n")

	if len(collector.spans) != 1 {
		t.Fatalf("completed spans = %d", len(collector.spans))
	}
	span := collector.spans[0]
	if span.SpanContext.TraceID() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("trace ID = %q, want the caller's", span.SpanContext.TraceID())
	}
	if span.ParentSpanID != "00f067aa0ba902b7" {
		t.Errorf("parent span ID = %q, want the caller's span", span.ParentSpanID)
	}
	if span.SpanContext.TraceState() != "vendor=value" {
		t.Errorf("tracestate = %q", span.SpanContext.TraceState())
	}
	if span.Kind != trace.SpanKindServer {
		t.Errorf("kind = %v, want server", span.Kind)
	}
}

// The refusals are the shared validator's, so this transport rejects exactly
// what the other one rejects. Two traceparents name two parents and the spec
// picks neither, which is the rule most easily lost to a Peek that takes the
// first value.
func TestOtelRefusesWhatTheSharedValidatorRefuses(t *testing.T) {
	for name, header := range map[string]string{
		"two traceparents": "Traceparent: " + remoteParent + "\r\nTraceparent: " + remoteParent + "\r\n",
		"uppercase hex":    "Traceparent: 00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01\r\n",
		"version ff":       "Traceparent: ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01\r\n",
		"zero span ID":     "Traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			collector := &spanCollector{}
			middleware := Otel(WithTracerProvider(trace.NewProvider(collector)))
			serveRaw(t, Chain(func(*fasthttp.RequestCtx) {}, middleware), "/orders", header)

			if len(collector.spans) != 1 {
				t.Fatalf("completed spans = %d", len(collector.spans))
			}
			if parent := collector.spans[0].ParentSpanID; parent != "" {
				t.Errorf("parent span ID = %q, want a root span", parent)
			}
		})
	}
}

// The request value is the context on this transport, so the span has to be
// readable straight off it — that is what a handler and every later frame use.
func TestOtelRecordsTheSpanOnTheRequestValue(t *testing.T) {
	collector := &spanCollector{}
	middleware := Otel(WithTracerProvider(trace.NewProvider(collector)))

	var sawTrace, sawSpan bool
	serveRaw(t, Chain(func(r *fasthttp.RequestCtx) {
		sawTrace = trace.SpanContextFromContext(r).TraceID() == "4bf92f3577b34da6a3ce929d0e0e4736"
		sawSpan = trace.SpanFromContext(r) != nil
		// A child opened from the request parents onto the server span without
		// the handler being handed a derived context.
		_, child := trace.Start(r, "load-order")
		child.End()
	}, middleware), "/orders", "Traceparent: "+remoteParent+"\r\n")

	if !sawTrace {
		t.Error("the handler could not read the trace off the request")
	}
	if !sawSpan {
		t.Error("the handler could not read the span off the request")
	}
	if len(collector.spans) != 2 {
		t.Fatalf("completed spans = %d, want the child and the server span", len(collector.spans))
	}
	child, server := collector.spans[0], collector.spans[1]
	if child.ParentSpanID != server.SpanContext.SpanID() {
		t.Errorf("child parent = %q, want the server span %q", child.ParentSpanID, server.SpanContext.SpanID())
	}
}

// The redaction is the shared one, so a token in a query string does not reach
// a trace backend from this transport either.
func TestOtelRedactsTheQueryString(t *testing.T) {
	collector := &spanCollector{}
	middleware := Otel(WithTracerProvider(trace.NewProvider(collector)))
	serveRaw(t, Chain(func(*fasthttp.RequestCtx) {}, middleware), "/orders?page=2&token=8f3c9a", "")

	if got, want := attributeOf(collector.spans[0], "url.query"), "page=REDACTED&token=REDACTED"; got != want {
		t.Errorf("url.query = %q, want %q", got, want)
	}
	if got := attributeOf(collector.spans[0], "url.path"); got != "/orders" {
		t.Errorf("url.path = %q", got)
	}
}

// A request with no traceparent is a root span rather than no span, which is
// what makes this process the start of a trace instead of a hole in one.
func TestOtelStartsARootSpanWithoutATraceparent(t *testing.T) {
	collector := &spanCollector{}
	middleware := Otel(WithTracerProvider(trace.NewProvider(collector)))
	serveRaw(t, Chain(func(*fasthttp.RequestCtx) {}, middleware), "/orders", "")

	if len(collector.spans) != 1 {
		t.Fatalf("completed spans = %d", len(collector.spans))
	}
	if collector.spans[0].ParentSpanID != "" {
		t.Errorf("parent = %q, want a root span", collector.spans[0].ParentSpanID)
	}
	if !collector.spans[0].SpanContext.IsValid() {
		t.Error("the root span has no usable span context")
	}
}

// A failed request is the one most worth having in a trace, so the span records
// the failure and still ends before the panic continues to Recover.
func TestOtelEndsTheSpanOnAPanic(t *testing.T) {
	collector := &spanCollector{}
	handler := Chain(func(*fasthttp.RequestCtx) { panic("handler failed") },
		Recover(nil), Otel(WithTracerProvider(trace.NewProvider(collector))))
	status, _, _ := serveRaw(t, handler, "/orders", "")

	if status != fasthttp.StatusInternalServerError {
		t.Fatalf("status = %d", status)
	}
	if len(collector.spans) != 1 {
		t.Fatalf("completed spans = %d, want the span ended rather than leaked", len(collector.spans))
	}
	if collector.spans[0].Status != trace.StatusError {
		t.Errorf("status = %v, want error", collector.spans[0].Status)
	}
}
