package propagation

import (
	"context"
	"net/http"
	"testing"

	"github.com/shibukawa/popcornwave/contrib/otel/trace"
)

func TestTraceContextExtractInject(t *testing.T) {
	header := http.Header{
		"Traceparent": {"00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"},
		"Tracestate":  {"vendor=value"},
	}
	ctx := (TraceContext{}).Extract(context.Background(), header)
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() || sc.TraceID() != "4bf92f3577b34da6a3ce929d0e0e4736" || sc.SpanID() != "00f067aa0ba902b7" || !sc.IsRemote() {
		t.Fatalf("unexpected span context: trace=%q span=%q remote=%v", sc.TraceID(), sc.SpanID(), sc.IsRemote())
	}
	out := make(http.Header)
	(TraceContext{}).Inject(ctx, out)
	if got := out.Get("traceparent"); got != header.Get("traceparent") {
		t.Fatalf("traceparent = %q", got)
	}
	if got := out.Get("tracestate"); got != "vendor=value" {
		t.Fatalf("tracestate = %q", got)
	}
}

// The core is what both transports share, so the rules that are expensive to
// get wrong are stated against it rather than against one transport's header.
func TestSpanContextFromFields(t *testing.T) {
	const valid = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

	// Two traceparents name two parents and the spec picks neither, so the
	// request has no parent rather than whichever one arrived first.
	if _, ok := SpanContextFromFields([]string{valid, valid}, nil); ok {
		t.Error("accepted two traceparent values")
	}
	if _, ok := SpanContextFromFields(nil, nil); ok {
		t.Error("accepted a request with no traceparent")
	}

	// Separate tracestate lines are one list, joined with commas.
	sc, ok := SpanContextFromFields([]string{valid}, []string{"a=1", "b=2"})
	if !ok || sc.TraceState() != "a=1,b=2" {
		t.Errorf("tracestate = %q, ok = %v", sc.TraceState(), ok)
	}

	// An unparseable tracestate costs the tracestate and not the parent: the
	// trace still joins up, and only the vendor data nobody here reads is lost.
	sc, ok = SpanContextFromFields([]string{valid}, []string{"Uppercase=1"})
	if !ok || !sc.IsValid() {
		t.Fatal("a bad tracestate dropped the parent")
	}
	if sc.TraceState() != "" {
		t.Errorf("tracestate = %q, want it dropped", sc.TraceState())
	}

	// A later version may append fields, and the delimiter is what separates a
	// forward-compatible sender from a corrupted one.
	if _, ok := SpanContextFromFields([]string{"01-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-extra"}, nil); !ok {
		t.Error("rejected a version 01 traceparent carrying an extra field")
	}
	if _, ok := SpanContextFromFields([]string{"01-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01extra"}, nil); ok {
		t.Error("accepted trailing bytes that no delimiter introduced")
	}
}

func TestFieldsSkipsAnInvalidSpanContext(t *testing.T) {
	if _, _, ok := Fields(trace.SpanContext{}); ok {
		t.Error("rendered fields for a process that is not tracing")
	}
}

func TestTraceContextRejectsMalformedParents(t *testing.T) {
	tests := []string{
		"00-00000000000000000000000000000000-00f067aa0ba902b7-01",
		"00-4BF92F3577B34DA6A3CE929D0E0E4736-00f067aa0ba902b7-01",
		"ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
		" 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	}
	for _, value := range tests {
		header := http.Header{"Traceparent": {value}}
		if got := trace.SpanContextFromContext((TraceContext{}).Extract(context.Background(), header)); got.IsValid() {
			t.Errorf("accepted %q", value)
		}
	}
}
