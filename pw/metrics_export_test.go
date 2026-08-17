package pw

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/middlewares"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// trackedChain is the two frames a metric needs: the tracker that carries the
// status, the byte count, and the route back up, and the frame that records.
func trackedChain(metrics *pwruntime.Metrics, next http.Handler) http.Handler {
	return middlewares.Track(middlewares.Metrics(metrics)(next))
}

// The end-to-end contract: a request served through the chain reaches the
// configured endpoint as http.server metrics, under a sampler that records no
// trace at all. That combination is the point of the feature — an instrument
// counts every request whatever the sampler kept.
func TestMetricsReachTheCollectorWithSamplingOff(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	received := newCollector(t)
	config := ObservabilityConfig{
		ServiceName: "metrics-test",
		Trace:       TraceConfig{Sampler: "always_off"},
		Metrics:     MetricsConfig{Enabled: "on", HTTP: true, Temporality: "delta"},
		Otel: OtelExportConfig{
			Endpoint:      received.server.URL,
			FlushInterval: 10 * time.Millisecond,
		},
	}
	resolved, err := buildObservability(config, EnvProduction)
	if err != nil {
		t.Fatal(err)
	}
	metrics := resolveMetrics(config, resolved.MetricProvider(), true)
	if metrics == nil {
		t.Fatal("no instruments were created for an enabled configuration")
	}

	handler := buildTestChain(t, metrics)
	for i := 0; i < 3; i++ {
		request := httptest.NewRequest(http.MethodGet, "/hello", nil)
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}

	// Shutdown collects and exports once more, so the assertion does not race
	// the reader's interval.
	if err := resolved.Shutdown(t.Context()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	body := received.body("/v1/metrics")
	if body == "" {
		t.Fatal("no metrics reached the collector")
	}
	for _, want := range []string{
		"http.server.request.duration",
		"http.server.active_requests",
		"metrics-test",
		`"http.route"`,
		"/hello",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the collection does not carry %s:\n%s", want, body)
		}
	}
	if traces := received.body("/v1/traces"); traces != "" {
		t.Errorf("always_off exported a trace:\n%s", traces)
	}
}

// buildTestChain wraps a handler in the metrics frame and a route that reports
// itself, which is what a generated page does through its response writer.
func buildTestChain(t *testing.T, metrics *pwruntime.Metrics) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		SetRoute(w, r)
		_, _ = w.Write([]byte("hello"))
	})
	return trackedChain(metrics, mux)
}

func TestRequestWithNoMatchedRouteCarriesNoRouteAttribute(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	received := newCollector(t)
	config := ObservabilityConfig{
		ServiceName: "metrics-route-test",
		Metrics:     MetricsConfig{Enabled: "on", HTTP: true, Temporality: "delta"},
		Otel:        OtelExportConfig{Endpoint: received.server.URL, FlushInterval: 10 * time.Millisecond},
	}
	resolved, err := buildObservability(config, EnvProduction)
	if err != nil {
		t.Fatal(err)
	}
	metrics := resolveMetrics(config, resolved.MetricProvider(), true)
	handler := trackedChain(metrics, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/unmatched/42", nil))
	if err := resolved.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
	body := received.body("/v1/metrics")
	if !strings.Contains(body, "http.server.request.duration") {
		t.Fatalf("no duration metric reached the collector:\n%s", body)
	}
	// Absent rather than raw: the unbounded value is exactly what the route
	// attribute exists to avoid.
	if strings.Contains(body, "/unmatched/42") {
		t.Errorf("the raw path reached a metric attribute:\n%s", body)
	}
	if strings.Contains(body, `"http.route"`) {
		t.Errorf("an unmatched request reported a route:\n%s", body)
	}
}
