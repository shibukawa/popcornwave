package pw

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/shibukawa/popcornwave/contrib/otel/trace"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// spanRecorder collects finished spans instead of exporting them.
type spanRecorder struct {
	mu    sync.Mutex
	spans []trace.SpanData
}

func (recorder *spanRecorder) OnEnd(span trace.SpanData) {
	recorder.mu.Lock()
	recorder.spans = append(recorder.spans, span)
	recorder.mu.Unlock()
}

func (recorder *spanRecorder) Shutdown(context.Context) error { return nil }

func (recorder *spanRecorder) ended() []trace.SpanData {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]trace.SpanData(nil), recorder.spans...)
}

// tracedRuntime installs a recording tracer provider for the test and restores
// the previous default afterwards, because the provider is process-wide.
func tracedRuntime(t *testing.T) *spanRecorder {
	t.Helper()
	previous := trace.DefaultProvider()
	recorder := &spanRecorder{}
	trace.SetDefaultProvider(trace.NewProvider(recorder))
	t.Cleanup(func() { trace.SetDefaultProvider(previous) })
	return recorder
}

// The framework opens the request root span, so a handler starts only the spans
// that describe its own work.
func TestRuntimeHandlerOpensARequestRootSpan(t *testing.T) {
	recorder := tracedRuntime(t)
	var traced bool
	handler, err := buildRuntimeHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traced = Traced(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}), ServerConfig{}, SecurityConfig{}, MiddlewareConfig{}, pwruntime.Resources{}, true)
	if err != nil {
		t.Fatal(err)
	}
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items", nil))

	if !traced {
		t.Fatal("the handler ran outside a trace")
	}
	spans := recorder.ended()
	if len(spans) != 1 {
		t.Fatalf("spans = %d, want the request root span", len(spans))
	}
	if spans[0].Kind != trace.SpanKindServer {
		t.Errorf("kind = %v, want a server span", spans[0].Kind)
	}
}

// With nothing exporting, no span is created at all: an unsampled span is pure
// cost on every request.
func TestRuntimeHandlerSkipsTracingWhenNothingExports(t *testing.T) {
	recorder := tracedRuntime(t)
	var traced bool
	handler, err := buildRuntimeHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traced = Traced(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}), ServerConfig{}, SecurityConfig{}, MiddlewareConfig{}, pwruntime.Resources{}, false)
	if err != nil {
		t.Fatal(err)
	}
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items", nil))

	if traced {
		t.Fatal("a span was created with tracing disabled")
	}
	if spans := recorder.ended(); len(spans) != 0 {
		t.Fatalf("spans = %d, want none", len(spans))
	}
}

// The payoff of sharing one attribute type and one context: a record taken
// inside a child span carries that span's IDs, so the viewer can put the log
// next to the span it came from.
func TestLoggerCorrelatesWithTheSpanItWasTakenFrom(t *testing.T) {
	recorder := tracedRuntime(t)
	sink := pwruntime.NewCaptureSink()
	resources := pwruntime.Resources{Log: pwruntime.NewLogBackend(LevelInfo, sink)}

	handler, err := buildRuntimeHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Logger(r.Context()).Info("in the root span")
		ctx, span := StartSpan(r.Context(), "load-user", String("db.system.name", "sqlite"))
		Logger(ctx).Info("in the child span", Int("rows", 3))
		span.End()
		w.WriteHeader(http.StatusNoContent)
	}), ServerConfig{}, SecurityConfig{}, MiddlewareConfig{}, resources, true)
	if err != nil {
		t.Fatal(err)
	}
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/users", nil))

	records := sink.Records()
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	root, child := records[0], records[1]
	if root.TraceID == "" || root.TraceID != child.TraceID {
		t.Fatalf("trace IDs = %q and %q, want one shared trace", root.TraceID, child.TraceID)
	}
	if root.SpanID == child.SpanID {
		t.Fatal("the child record was correlated with the root span")
	}
	spans := recorder.ended()
	if len(spans) != 2 {
		t.Fatalf("spans = %d, want the child and the root", len(spans))
	}
	// The first span to end is the child, and it is the one the second record
	// points at.
	if spans[0].SpanContext.SpanID() != child.SpanID {
		t.Errorf("child record span = %q, want %q", child.SpanID, spans[0].SpanContext.SpanID())
	}
	if spans[0].Name != "load-user" {
		t.Errorf("child span name = %q", spans[0].Name)
	}
}

// Outside a trace the accessors answer honestly rather than inventing IDs, and
// a span that records nothing is still safe to end.
func TestSpanAccessorsOutsideATrace(t *testing.T) {
	ctx := context.Background()
	if Traced(ctx) || TraceID(ctx) != "" || SpanID(ctx) != "" {
		t.Fatal("an untraced context reported trace identity")
	}
}
