package otlphttp

import (
	"context"
	"strconv"

	"github.com/shibukawa/popcornweb/contrib/otel/metric"
)

// OTLP aggregation temporality values.
const (
	temporalityDelta      = 1
	temporalityCumulative = 2
)

// ExportMetrics sends one collection to the metrics endpoint.
//
// It shares the endpoint, the headers, the client, the timeout, and the retry
// policy with the trace and log paths: this is one exporter with a third path
// rather than a second exporter. What it does not share is the queue, because a
// collection is not a stream of records and has nothing to bound.
func (e *Exporter) ExportMetrics(ctx context.Context, collected metric.ResourceData) error {
	if len(collected.Scopes) == 0 {
		return nil
	}
	scopes := make([]scopeMetrics, 0, len(collected.Scopes))
	for _, current := range collected.Scopes {
		encoded := scopeMetrics{Scope: scope{Name: current.Scope}}
		for _, data := range current.Metrics {
			encoded.Metrics = append(encoded.Metrics, encodeMetric(data))
		}
		scopes = append(scopes, encoded)
	}
	return e.send(ctx, e.metricsURL, metricRequest{ResourceMetrics: []resourceMetrics{{
		Resource:     resource{Attributes: attributes(collected.ResourceAttributes)},
		ScopeMetrics: scopes,
	}}})
}

func encodeMetric(data metric.Data) otlpMetric {
	encoded := otlpMetric{Name: data.Name, Unit: data.Unit, Description: data.Description}
	temporality := temporalityCumulative
	if data.Temporality == metric.DeltaTemporality {
		temporality = temporalityDelta
	}
	switch data.Kind {
	case metric.KindHistogram:
		points := make([]histogramDataPoint, 0, len(data.Histograms))
		for _, point := range data.Histograms {
			counts := make([]string, 0, len(point.BucketCounts))
			for _, count := range point.BucketCounts {
				counts = append(counts, strconv.FormatUint(count, 10))
			}
			sum, minimum, maximum := point.Sum, point.Min, point.Max
			points = append(points, histogramDataPoint{
				Attributes:        attributes(point.Attributes),
				StartTimeUnixNano: unixNano(point.Start), TimeUnixNano: unixNano(point.Time),
				Count: strconv.FormatUint(point.Count, 10), Sum: &sum,
				BucketCounts: counts, ExplicitBounds: point.Bounds,
				Min: &minimum, Max: &maximum,
			})
		}
		encoded.Histogram = &otlpHistogram{DataPoints: points, AggregationTemporality: temporality}
	case metric.KindGauge:
		// A gauge is sampled rather than accumulated, so it carries no
		// temporality: the point describes the instant it was read.
		encoded.Gauge = &otlpGauge{DataPoints: numberPoints(data.Numbers)}
	default:
		encoded.Sum = &otlpSum{
			DataPoints: numberPoints(data.Numbers), AggregationTemporality: temporality,
			IsMonotonic: data.Monotonic,
		}
	}
	return encoded
}

func numberPoints(input []metric.NumberPoint) []numberDataPoint {
	points := make([]numberDataPoint, 0, len(input))
	for _, point := range input {
		value := point.Value
		points = append(points, numberDataPoint{
			Attributes:        attributes(point.Attributes),
			StartTimeUnixNano: unixNano(point.Start), TimeUnixNano: unixNano(point.Time),
			AsDouble: &value,
		})
	}
	return points
}

// The uint64 fields are strings because that is the protobuf JSON mapping for a
// 64-bit integer, which is what the trace path already does for a timestamp.
type numberDataPoint struct {
	Attributes        []keyValue `json:"attributes,omitempty"`
	StartTimeUnixNano string     `json:"startTimeUnixNano,omitempty"`
	TimeUnixNano      string     `json:"timeUnixNano"`
	AsDouble          *float64   `json:"asDouble,omitempty"`
}
type histogramDataPoint struct {
	Attributes        []keyValue `json:"attributes,omitempty"`
	StartTimeUnixNano string     `json:"startTimeUnixNano,omitempty"`
	TimeUnixNano      string     `json:"timeUnixNano"`
	Count             string     `json:"count"`
	Sum               *float64   `json:"sum,omitempty"`
	BucketCounts      []string   `json:"bucketCounts,omitempty"`
	ExplicitBounds    []float64  `json:"explicitBounds,omitempty"`
	Min               *float64   `json:"min,omitempty"`
	Max               *float64   `json:"max,omitempty"`
}
type otlpSum struct {
	DataPoints             []numberDataPoint `json:"dataPoints"`
	AggregationTemporality int               `json:"aggregationTemporality,omitempty"`
	IsMonotonic            bool              `json:"isMonotonic,omitempty"`
}
type otlpGauge struct {
	DataPoints []numberDataPoint `json:"dataPoints"`
}
type otlpHistogram struct {
	DataPoints             []histogramDataPoint `json:"dataPoints"`
	AggregationTemporality int                  `json:"aggregationTemporality,omitempty"`
}
type otlpMetric struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Unit        string         `json:"unit,omitempty"`
	Sum         *otlpSum       `json:"sum,omitempty"`
	Gauge       *otlpGauge     `json:"gauge,omitempty"`
	Histogram   *otlpHistogram `json:"histogram,omitempty"`
}
type scopeMetrics struct {
	Scope   scope        `json:"scope"`
	Metrics []otlpMetric `json:"metrics"`
}
type resourceMetrics struct {
	Resource     resource       `json:"resource"`
	ScopeMetrics []scopeMetrics `json:"scopeMetrics"`
}
type metricRequest struct {
	ResourceMetrics []resourceMetrics `json:"resourceMetrics"`
}

// compile-time check that the exporter satisfies the reader's contract.
var _ metric.Exporter = (*Exporter)(nil)
