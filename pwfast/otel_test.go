package pwfast

import (
	gocontext "context"
	"strings"
	"testing"

	"github.com/shibukawa/popcornwave/contrib/otel"
	"github.com/shibukawa/popcornwave/contrib/otel/trace"
	"github.com/shibukawa/tinygodriver/fasthttp"
)

// A span reaches its readers through the context, and this transport cannot be
// handed a derived one — so the assertion is that the span is nevertheless
// where a reader looks.
func TestTheRequestSpanIsReachableFromTheRequestValue(t *testing.T) {
	var seen trace.SpanContext
	handler := Compose(func(r *fasthttp.RequestCtx) {
		seen = trace.SpanContextFromContext(r)
	}, Frame{Slot: SlotTracing, Middleware: Otel()})
	serve(t, handler, "/orders")

	if !seen.IsValid() {
		t.Error("the handler found no span context on the request")
	}
}

// An incoming trace continues rather than starting a second one, which is the
// whole point of propagation.
func TestAnIncomingTraceIsContinued(t *testing.T) {
	const parent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	var seen trace.SpanContext
	handler := Compose(func(r *fasthttp.RequestCtx) {
		seen = trace.SpanContextFromContext(r)
	}, Frame{Slot: SlotTracing, Middleware: Otel()})
	serveRaw(t, handler, "/orders", "traceparent: "+parent+"\r\n")

	if got := seen.TraceID(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("trace id = %q, want the caller's", got)
	}
}

// A query string must never reach a trace backend verbatim: it is where a
// password-reset token, an OAuth code and a presigned signature all travel.
func TestTheQueryIsRecordedWithoutItsValues(t *testing.T) {
	attributes := requestAttributesOf(t, "/reset?token=super-secret&page=2")
	for _, attribute := range attributes {
		if attribute.Key != "url.query" {
			continue
		}
		value, _ := attribute.Value.AsString()
		if strings.Contains(value, "super-secret") {
			t.Errorf("the span carried a secret query value: %q", value)
		}
		if !strings.Contains(value, "token") || !strings.Contains(value, "page") {
			t.Errorf("the span lost the parameter names, which are the diagnostic value: %q", value)
		}
		return
	}
	t.Error("no url.query attribute was recorded")
}

func requestAttributesOf(t *testing.T, target string) []otelAttribute {
	t.Helper()
	var captured []otelAttribute
	handler := func(r *fasthttp.RequestCtx) { captured = requestAttributes(r) }
	serve(t, handler, target)
	return captured
}

// otelAttribute names the attribute type locally so the helper above reads
// without repeating the import path.
type otelAttribute = otel.Attribute

type spanCollector struct{ spans []trace.SpanData }

func (c *spanCollector) OnEnd(span trace.SpanData)      { c.spans = append(c.spans, span) }
func (*spanCollector) Shutdown(gocontext.Context) error { return nil }

const remoteParent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

// The refusals belong to the shared validator, so this transport must reject
// exactly what the other one rejects.
//
// Two traceparents is the case that decides whether the reading is honest. The
// header is read with PeekAll for this: Peek returns the first value, which
// would hand the validator one field and get a parent accepted here that the
// other transport refuses.
func TestTheRefusalsMatchTheOtherTransport(t *testing.T) {
	for name, header := range map[string]string{
		"two traceparents": "traceparent: " + remoteParent + "\r\ntraceparent: " + remoteParent + "\r\n",
		"uppercase hex":    "traceparent: 00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01\r\n",
		"version ff":       "traceparent: ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01\r\n",
		"zero span id":     "traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			collector := &spanCollector{}
			handler := Compose(func(*fasthttp.RequestCtx) {},
				Frame{Slot: SlotTracing, Middleware: Otel(WithTracerProvider(trace.NewProvider(collector)))})
			serveRaw(t, handler, "/orders", header)

			if len(collector.spans) != 1 {
				t.Fatalf("completed spans = %d", len(collector.spans))
			}
			if parent := collector.spans[0].ParentSpanID; parent != "" {
				t.Errorf("parent = %q, want the request refused a parent and rooted its own trace", parent)
			}
		})
	}
}

// A separate tracestate line survives the reading, which Peek would also have
// truncated.
func TestTracestateIsCarried(t *testing.T) {
	var seen trace.SpanContext
	handler := Compose(func(r *fasthttp.RequestCtx) { seen = trace.SpanContextFromContext(r) },
		Frame{Slot: SlotTracing, Middleware: Otel()})
	serveRaw(t, handler, "/orders", "traceparent: "+remoteParent+"\r\ntracestate: a=1\r\ntracestate: b=2\r\n")

	if seen.TraceState() != "a=1,b=2" {
		t.Errorf("tracestate = %q, want both lines joined", seen.TraceState())
	}
}

// A child opened from the request parents onto the request span, without the
// handler ever being handed a derived context.
func TestAChildSpanParentsOntoTheRequestSpan(t *testing.T) {
	collector := &spanCollector{}
	handler := Compose(func(r *fasthttp.RequestCtx) {
		_, child := trace.Start(r, "load-order")
		child.End()
	}, Frame{Slot: SlotTracing, Middleware: Otel(WithTracerProvider(trace.NewProvider(collector)))})
	serve(t, handler, "/orders")

	if len(collector.spans) != 2 {
		t.Fatalf("completed spans = %d, want the child and the request span", len(collector.spans))
	}
	child, request := collector.spans[0], collector.spans[1]
	if child.ParentSpanID != request.SpanContext.SpanID() {
		t.Errorf("child parent = %q, want the request span %q", child.ParentSpanID, request.SpanContext.SpanID())
	}
}

// A failed request is the one most worth having in a trace, so the span ends
// rather than leaking and it says the request failed.
func TestTheSpanEndsOnAPanic(t *testing.T) {
	collector := &spanCollector{}
	handler := Compose(func(*fasthttp.RequestCtx) { panic("handler failed") },
		Frame{Slot: SlotRecover, Middleware: Recover(nil)},
		Frame{Slot: SlotTracing, Middleware: Otel(WithTracerProvider(trace.NewProvider(collector)))})
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
