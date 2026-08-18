package pwobservability

import (
	"fmt"
	"strings"

	"github.com/shibukawa/popcornweb/contrib/otel/trace"
	"github.com/shibukawa/popcornweb/pwconfig"
	"github.com/shibukawa/popcornweb/pwruntime"
)

// TracingPolicy turns configuration into the runtime span policy, or nil when
// the framework should open no span of its own.
//
// exporting is the automatic answer: whether this process has somewhere to send
// a span. It is the same signal that decides whether the request root span is
// created, so auto keeps the two in step — a run with no collector opens no
// root and no children, and the dev loop, which is handed an endpoint, gets the
// whole tree without configuring anything.
//
// An invalid value resolves to nil here; validateTraceConfig reports it before
// any request is served, exactly as query diagnostics do.
func TracingPolicy(config pwconfig.ObservabilityConfig, exporting bool) *pwruntime.Tracing {
	enabled, err := ResolveToggle(config.Trace.Enabled, exporting)
	if err != nil || !enabled {
		return nil
	}
	if !config.Trace.Render && !config.Trace.Database {
		// Every switch below is off, so the policy would be a value that creates
		// nothing. Returning nil instead is what makes the cost one comparison
		// rather than several.
		return nil
	}
	return &pwruntime.Tracing{
		Render:   config.Trace.Render,
		Boundary: config.Trace.Render && config.Trace.Boundary,
		Database: config.Trace.Database,
		// The text is what makes a database span readable, and it is also the
		// one attribute that can be large, so it takes the bound query
		// diagnostics already declares rather than a second one to keep in step.
		Statement:    config.Trace.Database && config.Trace.Statement,
		MaxSQLLength: positiveOr(config.Query.MaxSQLLength, defaultQueryMaxSQLLength),
	}
}

// ResolveSampler builds the head sampler for this process.
//
// An empty configured name takes the environment's default rather than one value
// everywhere: development records every trace because the loop's only view of a
// request is the trace it kept, and every other environment samples because the
// process may be exporting straight to a backend that bills per span. Parse
// normally fills the key before this runs, so that the startup summary can name
// the provenance; the resolution is repeated here for the path that never parses
// a configuration at all.
func ResolveSampler(config pwconfig.ObservabilityConfig, env string) (trace.Sampler, error) {
	name, argument := strings.TrimSpace(config.Trace.Sampler), strings.TrimSpace(config.Trace.SamplerArg)
	if name == "" {
		name, argument = pwconfig.DefaultTraceSampler(env)
	}
	sampler, err := trace.ParseSampler(name, argument)
	if err != nil {
		return nil, fmt.Errorf("observability.trace.sampler: %w", err)
	}
	return sampler, nil
}

// TraceForced reports whether configuration asked for framework spans outright
// rather than through auto.
//
// It is what installs the request root span in a process that exports nothing.
// Without it, "on" would produce a render span and a statement span with no
// parent between them and no server span above, which is a set of disconnected
// roots rather than a trace. A project reaching this state is one holding its
// own provider, and it wants the whole tree.
func TraceForced(config pwconfig.ObservabilityConfig) bool {
	enabled, err := ResolveToggle(config.Trace.Enabled, false)
	return err == nil && enabled
}
