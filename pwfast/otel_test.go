package pwfast

import (
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
