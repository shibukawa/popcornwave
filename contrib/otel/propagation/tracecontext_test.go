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
