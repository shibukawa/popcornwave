package pw

import (
	"github.com/shibukawa/popcornwave/contrib/otel/metric"
	"github.com/shibukawa/popcornwave/pwobservability"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// The observability layer lives in popcornwave/pwobservability, and every entry
// below is a thin wrapper over one declared there.
//
// It moved because none of it is transport-shaped: the emission policy, the
// OTLP exporters, the query diagnostics and the span policy are all resolved
// from configuration and would be resolved identically by a runtime serving on
// another transport. What stays here is the startup summary, which reads the
// result, and the request frames, which are each transport's own.

// Stdout record formats.
const (
	StdoutFormatJSON      = pwobservability.StdoutFormatJSON
	StdoutFormatPlaintext = pwobservability.StdoutFormatPlaintext
)

// Diagnostic toggles. Auto ties the setting to something the process already
// knows, so a run that wants the ordinary answer configures nothing: query
// diagnostics read the runtime environment, and framework spans read whether
// anything exports them.
const (
	QueryToggleAuto = pwobservability.QueryToggleAuto
	QueryToggleOn   = pwobservability.QueryToggleOn
	QueryToggleOff  = pwobservability.QueryToggleOff
)

// observability is the resolved emission and tracing state of one process.
type observability = pwobservability.Resolved

// buildObservability resolves configuration and the standard OTLP environment
// into one emission policy, and registers what it opened for shutdown.
//
// Registering the closers is this runtime's rather than the layer's: shutdown
// order is a property of the chain that was built, and a log sink has to
// outlive whatever still logs into it.
func buildObservability(config ObservabilityConfig, env string) (*observability, error) {
	resolved, err := pwobservability.Build(config, env)
	if err != nil {
		return nil, err
	}
	for _, cleanup := range resolved.Cleanups() {
		registerCleanup(cleanup.Name, cleanup.Close)
	}
	return resolved, nil
}

// processLogger returns a logger for framework output that belongs to the
// process rather than to a request. Before configuration is parsed it falls
// back to the same stderr logger an unconfigured context gets.
func processLogger() Log { return pwobservability.ProcessLogger() }

// resolveQueryDiagnostics turns configuration into the runtime setting, or nil
// when query diagnostics are off.
func resolveQueryDiagnostics(config ObservabilityConfig, development bool) *pwruntime.QueryDiagnostics {
	return pwobservability.QueryDiagnostics(config, development)
}

func reportQueryDiagnostics(diagnostics *pwruntime.QueryDiagnostics, env string, development bool, driver string) {
	pwobservability.ReportQueryDiagnostics(diagnostics, env, development, driver)
}

// resolveTracing turns configuration into the runtime span policy, or nil when
// the framework should open no span of its own.
func resolveTracing(config ObservabilityConfig, exporting bool) *pwruntime.Tracing {
	return pwobservability.TracingPolicy(config, exporting)
}

// resolveMetrics turns configuration into the runtime instrument set. exporting
// is the automatic answer, the same one tracing reads, and the only thing the two
// share: a sampler that keeps almost nothing changes no count here.
func resolveMetrics(config ObservabilityConfig, provider *metric.Provider, exporting bool) *pwruntime.Metrics {
	return pwobservability.MetricsPolicy(config, provider, exporting)
}

// traceForced reports whether configuration asked for framework spans outright
// rather than through auto.
func traceForced(config ObservabilityConfig) bool { return pwobservability.TraceForced(config) }

func resolveToggle(value string, auto bool) (bool, error) {
	return pwobservability.ResolveToggle(value, auto)
}
