package pwobservability

import (
	"fmt"
	"strings"

	"github.com/shibukawa/popcornweb/contrib/otel/metric"
	"github.com/shibukawa/popcornweb/pwconfig"
	"github.com/shibukawa/popcornweb/pwruntime"
)

// Metric temporality tokens.
const (
	TemporalityDelta      = "delta"
	TemporalityCumulative = "cumulative"
)

// MetricsPolicy turns configuration into the runtime instrument set, or nil when
// the framework should record nothing.
//
// exporting is the automatic answer, the same signal the trace policy reads: an
// aggregation nothing exports is pure cost. What it does not read is the trace
// policy itself — a process sampling one trace in a thousand still counts every
// request, and that independence is the reason both signals exist.
//
// An invalid toggle resolves to nil here; validateMetricsConfig reports it before
// any request is served, exactly as the trace toggle does.
func MetricsPolicy(config pwconfig.ObservabilityConfig, provider *metric.Provider, exporting bool) *pwruntime.Metrics {
	enabled, err := ResolveToggle(config.Metrics.Enabled, exporting)
	if err != nil || !enabled || provider == nil {
		return nil
	}
	groups := pwruntime.MetricGroups{
		HTTP:   config.Metrics.HTTP,
		DB:     config.Metrics.DB,
		Render: config.Metrics.Render,
		Cache:  config.Metrics.Cache,
	}
	// Every group off is a policy that would create nothing, so returning nil
	// keeps the cost one comparison rather than several false branches. The
	// runtime group is not here: it registers callbacks rather than being
	// recorded on a request path, so it is not part of this set.
	return pwruntime.NewMetrics(provider.Meter(pwruntime.MetricScope), groups)
}

// ParseTemporality maps the configured token to the exported temporality.
func ParseTemporality(value string) (metric.Temporality, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", TemporalityDelta:
		return metric.DeltaTemporality, nil
	case TemporalityCumulative:
		return metric.CumulativeTemporality, nil
	default:
		return 0, fmt.Errorf("must be %s or %s", TemporalityDelta, TemporalityCumulative)
	}
}

// MetricsForced reports whether configuration asked for instruments outright
// rather than through auto, which is what lets a project holding its own reader
// record without configuring an endpoint here.
func MetricsForced(config pwconfig.ObservabilityConfig) bool {
	enabled, err := ResolveToggle(config.Metrics.Enabled, false)
	return err == nil && enabled
}
