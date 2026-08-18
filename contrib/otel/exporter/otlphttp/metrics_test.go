package otlphttp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/contrib/otel"
	"github.com/shibukawa/popcornweb/contrib/otel/metric"
)

// TestExportMetricsEncodesTheOTLPShape asserts the field names and the string
// encoding of the 64-bit fields, because a receiver decodes this with protobuf
// JSON and a wrong spelling is accepted by nothing and reported by no test but
// this one.
func TestExportMetricsEncodesTheOTLPShape(t *testing.T) {
	var body []byte
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exporter, err := New(Config{Endpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	collected := metric.ResourceData{
		ResourceAttributes: []otel.Attribute{otel.String("service.name", "metrics-test")},
		Scopes: []metric.ScopeData{{Scope: "test-scope", Metrics: []metric.Data{
			{
				Name: "http.server.request.duration", Unit: "s", Kind: metric.KindHistogram,
				Temporality: metric.DeltaTemporality,
				Histograms: []metric.HistogramPoint{{
					Attributes: []otel.Attribute{otel.Int64("http.response.status_code", 200)},
					Count:      2, Sum: 0.3, Min: 0.1, Max: 0.2,
					Bounds:       []float64{0.15},
					BucketCounts: []uint64{1, 1},
					Start:        now, Time: now,
				}},
			},
			{
				Name: "http.server.active_requests", Unit: "{request}", Kind: metric.KindUpDownCounter,
				Temporality: metric.CumulativeTemporality,
				Numbers:     []metric.NumberPoint{{Value: 3, Start: now, Time: now}},
			},
			{
				Name: "pw.data_cache.entries", Unit: "{entry}", Kind: metric.KindGauge,
				Numbers: []metric.NumberPoint{{Value: 7, Time: now}},
			},
		}}},
	}
	if err := exporter.ExportMetrics(context.Background(), collected); err != nil {
		t.Fatal(err)
	}
	if path != "/v1/metrics" {
		t.Fatalf("path = %q", path)
	}

	var decoded struct {
		ResourceMetrics []struct {
			Resource struct {
				Attributes []struct{ Key string } `json:"attributes"`
			} `json:"resource"`
			ScopeMetrics []struct {
				Scope   struct{ Name string } `json:"scope"`
				Metrics []struct {
					Name      string `json:"name"`
					Unit      string `json:"unit"`
					Histogram *struct {
						AggregationTemporality int `json:"aggregationTemporality"`
						DataPoints             []struct {
							Count          string    `json:"count"`
							Sum            *float64  `json:"sum"`
							BucketCounts   []string  `json:"bucketCounts"`
							ExplicitBounds []float64 `json:"explicitBounds"`
							TimeUnixNano   string    `json:"timeUnixNano"`
						} `json:"dataPoints"`
					} `json:"histogram"`
					Sum *struct {
						AggregationTemporality int  `json:"aggregationTemporality"`
						IsMonotonic            bool `json:"isMonotonic"`
						DataPoints             []struct {
							AsDouble *float64 `json:"asDouble"`
						} `json:"dataPoints"`
					} `json:"sum"`
					Gauge *struct {
						DataPoints []struct {
							AsDouble *float64 `json:"asDouble"`
						} `json:"dataPoints"`
					} `json:"gauge"`
				} `json:"metrics"`
			} `json:"scopeMetrics"`
		} `json:"resourceMetrics"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode: %v\n%s", err, body)
	}
	if len(decoded.ResourceMetrics) != 1 {
		t.Fatalf("resourceMetrics = %d", len(decoded.ResourceMetrics))
	}
	resourceMetric := decoded.ResourceMetrics[0]
	if got := resourceMetric.Resource.Attributes[0].Key; got != "service.name" {
		t.Errorf("resource attribute = %q", got)
	}
	if got := resourceMetric.ScopeMetrics[0].Scope.Name; got != "test-scope" {
		t.Errorf("scope = %q", got)
	}
	metrics := resourceMetric.ScopeMetrics[0].Metrics
	if len(metrics) != 3 {
		t.Fatalf("metrics = %d", len(metrics))
	}
	histogram := metrics[0]
	if histogram.Histogram == nil {
		t.Fatalf("%s did not encode as a histogram", histogram.Name)
	}
	if histogram.Histogram.AggregationTemporality != temporalityDelta {
		t.Errorf("temporality = %d, want delta", histogram.Histogram.AggregationTemporality)
	}
	point := histogram.Histogram.DataPoints[0]
	if point.Count != "2" {
		t.Errorf("count = %q, want a string-encoded uint64", point.Count)
	}
	if point.Sum == nil || *point.Sum != 0.3 {
		t.Errorf("sum = %v", point.Sum)
	}
	if len(point.BucketCounts) != 2 || point.BucketCounts[0] != "1" {
		t.Errorf("bucketCounts = %v", point.BucketCounts)
	}
	if len(point.ExplicitBounds) != 1 || point.ExplicitBounds[0] != 0.15 {
		t.Errorf("explicitBounds = %v", point.ExplicitBounds)
	}
	if point.TimeUnixNano != "1700000000000000000" {
		t.Errorf("timeUnixNano = %q", point.TimeUnixNano)
	}
	active := metrics[1]
	if active.Sum == nil {
		t.Fatalf("%s did not encode as a sum", active.Name)
	}
	if active.Sum.IsMonotonic {
		t.Error("an up_down_counter encoded as monotonic")
	}
	if active.Sum.AggregationTemporality != temporalityCumulative {
		t.Errorf("temporality = %d, want cumulative", active.Sum.AggregationTemporality)
	}
	if value := active.Sum.DataPoints[0].AsDouble; value == nil || *value != 3 {
		t.Errorf("value = %v", value)
	}
	gauge := metrics[2]
	if gauge.Gauge == nil {
		t.Fatalf("%s did not encode as a gauge", gauge.Name)
	}
	if value := gauge.Gauge.DataPoints[0].AsDouble; value == nil || *value != 7 {
		t.Errorf("gauge value = %v", value)
	}
}

func TestExportMetricsSkipsAnEmptyCollection(t *testing.T) {
	exporter, err := New(Config{Endpoint: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	// No request is attempted, so an unreachable endpoint is not an error here.
	if err := exporter.ExportMetrics(context.Background(), metric.ResourceData{}); err != nil {
		t.Fatal(err)
	}
}
