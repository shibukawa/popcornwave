package pw

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/pwruntime"
)

// collector stands in for an OTLP/HTTP receiver and keeps the bodies it was
// sent, so a test can assert on what actually left the process.
type collector struct {
	mu     sync.Mutex
	bodies map[string][]byte
	server *httptest.Server
}

func newCollector(t *testing.T) *collector {
	t.Helper()
	received := &collector{bodies: map[string][]byte{}}
	received.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received.mu.Lock()
		received.bodies[r.URL.Path] = append(received.bodies[r.URL.Path], body...)
		received.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(received.server.Close)
	return received
}

func (received *collector) body(path string) string {
	received.mu.Lock()
	defer received.mu.Unlock()
	return string(received.bodies[path])
}

// The end-to-end contract of the whole feature: a span opened through the
// framework and a record written through the framework logger both reach the
// configured endpoint, carrying one shared trace ID.
func TestObservabilityExportsSpansAndCorrelatedLogs(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	received := newCollector(t)
	resolved, err := buildObservability(ObservabilityConfig{
		ServiceName: "export-test",
		// The sampler is named because this test is about export rather than
		// about sampling: a production environment samples by default, and one
		// span at that ratio is a coin toss.
		Trace: TraceConfig{Sampler: "always_on"},
		Otel: OtelExportConfig{
			Endpoint:      received.server.URL,
			FlushInterval: 10 * time.Millisecond,
		},
	}, EnvProduction)
	if err != nil {
		t.Fatal(err)
	}

	ctx, span := StartSpanContext(t.Context(), "load-user", String("db.system.name", "sqlite"))
	pwruntime.NewLogger(ctx, resolved.Backend()).Info("user loaded", Int("rows", 3))
	span.End()

	// Shutdown flushes both providers, so the assertions do not race the batch
	// interval.
	if err := resolved.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	traces, logs := received.body("/v1/traces"), received.body("/v1/logs")
	if !strings.Contains(traces, "load-user") {
		t.Fatalf("the span never reached the collector:\n%s", traces)
	}
	if !strings.Contains(traces, "db.system.name") {
		t.Errorf("span attributes were not exported:\n%s", traces)
	}
	if !strings.Contains(logs, "user loaded") {
		t.Fatalf("the record never reached the collector:\n%s", logs)
	}
	if !strings.Contains(logs, "export-test") || !strings.Contains(traces, "export-test") {
		t.Error("the service name was not reported to the collector")
	}

	// One trace ID appears in both payloads, which is what lets a viewer put the
	// record next to the span it came from.
	traceID := TraceIDContext(ctx)
	if traceID == "" {
		t.Fatal("the started span produced no trace ID")
	}
	if !strings.Contains(traces, traceID) || !strings.Contains(logs, traceID) {
		t.Fatalf("trace %s is not in both payloads:\ntraces=%s\nlogs=%s", traceID, traces, logs)
	}
}

// A production run configured for OTLP writes nothing to stdout, so the same
// record is not paid for twice.
func TestProductionExportLeavesStdoutAlone(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	received := newCollector(t)
	resolved, err := buildObservability(ObservabilityConfig{
		Otel: OtelExportConfig{Endpoint: received.server.URL, FlushInterval: 10 * time.Millisecond},
	}, EnvProduction)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resolved.Shutdown(t.Context()) })
	if resolved.SinkCount() != 1 {
		t.Fatalf("sinks = %d, want the collector alone", resolved.SinkCount())
	}
}
