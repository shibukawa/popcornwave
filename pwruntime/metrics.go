package pwruntime

import (
	"context"
	"fmt"
	"strings"

	"github.com/shibukawa/popcornwave/contrib/otel/metric"
)

// MetricScope names the instrumentation scope of framework instruments, matching
// the scope its spans already carry.
const MetricScope = "github.com/shibukawa/popcornwave"

// Metrics is the resolved framework instrument set installed by pw.
//
// A nil value records nothing, and so does a nil field inside it: a group the
// configuration declined leaves its instruments nil, and every recording site is
// one nil comparison. It mirrors Tracing in being a resolved policy rather than
// the configuration struct, and it differs from Tracing in one way that matters
// — a recording here never consults whether a span is being sampled, because a
// count of the sampled fraction is not a count.
type Metrics struct {
	// RequestDuration is http.server.request.duration. Its count answers how
	// many requests were served and its buckets answer the percentile, so no
	// counter sits beside it.
	RequestDuration *metric.Histogram
	// ActiveRequests is http.server.active_requests, the concurrency no
	// per-request record shows.
	ActiveRequests *metric.UpDownCounter
	// RequestBodySize and ResponseBodySize are the http.server body size
	// histograms. A request with no declared length records nothing rather than
	// zero.
	RequestBodySize  *metric.Histogram
	ResponseBodySize *metric.Histogram

	// QueryDuration is db.client.operation.duration, recorded on the same
	// resolver seam the statement span uses.
	QueryDuration *metric.Histogram

	// RenderDuration and RenderBytes describe one response per render mode,
	// which is the branch nothing outside this process can attribute a response
	// time to.
	RenderDuration *metric.Histogram
	RenderBytes    *metric.Histogram
	// BoundarySettle is how long a fallback held the screen. It carries no
	// boundary id: positional is safe on a span and unbounded across pages.
	BoundarySettle *metric.Histogram
	// LiveDelivery is the interval between consecutive deliveries of one
	// boundary, and LiveActive is how many subscriptions exist right now.
	LiveDelivery *metric.Histogram
	LiveActive   *metric.UpDownCounter
	LiveClosed   *metric.Counter

	// RenderCache counts component output cache operations, with a result
	// attribute rather than one instrument per outcome, because a reader
	// dividing them needs both under one name.
	RenderCache *metric.Counter
	// DataCache counts data result cache operations the same way.
	DataCache *metric.Counter
}

// MetricGroups selects which instruments exist. A false group leaves its fields
// nil, which is what makes declining a group free rather than cheap.
type MetricGroups struct {
	HTTP   bool
	DB     bool
	Render bool
	Cache  bool
}

// NewMetrics creates the instrument set for one meter.
//
// Every name, unit, and attribute set of the http and db groups is the semantic
// convention's; the pw-prefixed ones are this framework's, one per question no
// external agent can answer.
func NewMetrics(meter *metric.Meter, groups MetricGroups) *Metrics {
	if meter == nil || groups == (MetricGroups{}) {
		return nil
	}
	metrics := &Metrics{}
	if groups.HTTP {
		metrics.RequestDuration = meter.Histogram("http.server.request.duration", "s",
			"Duration of HTTP server requests.", metric.DurationBounds)
		metrics.ActiveRequests = meter.UpDownCounter("http.server.active_requests", "{request}",
			"Number of active HTTP server requests.")
		metrics.RequestBodySize = meter.Histogram("http.server.request.body.size", "By",
			"Size of HTTP server request bodies.", metric.SizeBounds)
		metrics.ResponseBodySize = meter.Histogram("http.server.response.body.size", "By",
			"Size of HTTP server response bodies.", metric.SizeBounds)
	}
	if groups.DB {
		metrics.QueryDuration = meter.Histogram("db.client.operation.duration", "s",
			"Duration of database client operations.", metric.DurationBounds)
	}
	if groups.Render {
		metrics.RenderDuration = meter.Histogram("pw.render.duration", "s",
			"Duration of one rendered response, by render mode.", metric.DurationBounds)
		metrics.RenderBytes = meter.Histogram("pw.render.bytes", "By",
			"Uncompressed bytes that reached the client, by render mode.", metric.SizeBounds)
		metrics.BoundarySettle = meter.Histogram("pw.boundary.settle.duration", "s",
			"How long an await boundary's fallback held the screen.", metric.DurationBounds)
		metrics.LiveDelivery = meter.Histogram("pw.live.delivery.duration", "s",
			"Interval between consecutive deliveries of one live boundary.", metric.DurationBounds)
		metrics.LiveActive = meter.UpDownCounter("pw.live.subscriptions.active", "{subscription}",
			"Live subscriptions currently open.")
		metrics.LiveClosed = meter.Counter("pw.live.closed", "{response}",
			"Live responses closed, by close reason.")
	}
	if groups.Cache {
		metrics.RenderCache = meter.Counter("pw.render.cache.operations", "{operation}",
			"Component output cache lookups, by result.")
		metrics.DataCache = meter.Counter("pw.data_cache.operations", "{operation}",
			"Data result cache lookups, by result.")
	}
	return metrics
}

// MetricPolicy returns the framework instrument set of ctx, or nil when nothing
// is recorded. It is one context lookup, the same one every other resource
// accessor makes.
func MetricPolicy(ctx context.Context) *Metrics { return resources(ctx).Metrics }

// Cache result attribute values, a closed set so that a hit rate has a
// denominator and a reader can enumerate the outcomes.
const (
	CacheResultHit       = "hit"
	CacheResultMiss      = "miss"
	CacheResultStaleHit  = "stale_hit"
	CacheResultCoalesced = "coalesced"
)

// ErrorType is the error.type attribute value for an error.
//
// It reports the type name and never the message. A message carries values — a
// path, an identifier, a bind value policy:query-log-safety keeps off a span —
// and an attribute built from one is an unbounded series wearing a closed set's
// name.
func ErrorType(err error) string {
	if err == nil {
		return ""
	}
	name := strings.TrimPrefix(fmt.Sprintf("%T", err), "*")
	if name == "" {
		return "_OTHER"
	}
	return name
}
