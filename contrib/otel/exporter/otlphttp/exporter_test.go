package otlphttp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/contrib/otel"
	otellog "github.com/shibukawa/popcornwave/contrib/otel/log"
	"github.com/shibukawa/popcornwave/contrib/otel/trace"
)

func TestExportTraceAndStandaloneLogJSON(t *testing.T) {
	var mu sync.Mutex
	requests := make(map[string]map[string]any)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode: %v", err)
		}
		mu.Lock()
		requests[r.URL.Path] = body
		mu.Unlock()
		return response(http.StatusOK, ""), nil
	})}

	exporter, err := New(Config{Endpoint: "http://collector.test:4318", Headers: http.Header{"Authorization": {"Bearer test"}}, Client: client, MaxRetries: -1})
	if err != nil {
		t.Fatal(err)
	}
	parent, err := trace.NewSpanContext("4bf92f3577b34da6a3ce929d0e0e4736", "00f067aa0ba902b7", 1, "vendor=value", true)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1, 2)
	span := trace.SpanData{Name: "GET", SpanContext: parent, Kind: trace.SpanKindServer, StartTime: now, EndTime: now.Add(time.Millisecond), Attributes: []otel.Attribute{otel.Int64("http.response.status_code", 200)}, ScopeName: "test", ResourceAttributes: []otel.Attribute{otel.String("service.name", "api")}}
	if err := exporter.ExportSpans(context.Background(), []trace.SpanData{span}); err != nil {
		t.Fatal(err)
	}
	record := otellog.RecordData{Record: otellog.Record{Timestamp: now, ObservedTime: now, Severity: otellog.SeverityInfo, Body: "ready", EventName: "app.ready"}, ScopeName: "test"}
	if err := exporter.ExportLogs(context.Background(), []otellog.RecordData{record}); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if requests["/v1/traces"] == nil || requests["/v1/logs"] == nil {
		t.Fatalf("request paths = %v", requests)
	}
	resourceSpans := requests["/v1/traces"]["resourceSpans"].([]any)
	encodedSpan := resourceSpans[0].(map[string]any)["scopeSpans"].([]any)[0].(map[string]any)["spans"].([]any)[0].(map[string]any)
	if encodedSpan["traceId"] != parent.TraceID() || encodedSpan["startTimeUnixNano"] != "1000000002" {
		t.Fatalf("span JSON = %#v", encodedSpan)
	}
	resourceLogs := requests["/v1/logs"]["resourceLogs"].([]any)
	encodedLog := resourceLogs[0].(map[string]any)["scopeLogs"].([]any)[0].(map[string]any)["logRecords"].([]any)[0].(map[string]any)
	if _, exists := encodedLog["traceId"]; exists {
		t.Fatalf("standalone log has traceId: %#v", encodedLog)
	}
}

func TestExporterRetriesTransientStatus(t *testing.T) {
	attempts := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return response(http.StatusServiceUnavailable, "retry"), nil
		}
		return response(http.StatusOK, ""), nil
	})}
	exporter, err := New(Config{Endpoint: "http://collector.test:4318", Client: client, MaxRetries: 1, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	sc, _ := trace.NewSpanContext("4bf92f3577b34da6a3ce929d0e0e4736", "00f067aa0ba902b7", 1, "", false)
	if err := exporter.ExportSpans(ctx, []trace.SpanData{{Name: "test", SpanContext: sc, StartTime: time.Now(), EndTime: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d", attempts)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
