package pwruntime

import "context"

// Tracing is the resolved framework span policy installed by pw.
//
// A nil value creates no framework span at all, which is what an application
// exporting nothing pays for the feature: one nil comparison per render and per
// statement, and no span object anywhere. It is a resolved policy rather than
// the configuration struct for the same reason QueryDiagnostics is — the
// auto/on/off vocabulary and the environment are read once at startup, not once
// per request.
type Tracing struct {
	// Render opens a span for one HTML response, with the initial build as its
	// first child.
	Render bool
	// Boundary opens a span per settled await boundary and per live delivery.
	// It depends on Render, because a boundary span with no render span to sit
	// under would attach each fragment straight to the request root.
	Boundary bool
	// Database opens a client span per executed statement.
	Database bool
	// Statement puts the statement text on that span. Bind values never reach
	// it: policy:query-log-safety keeps row data in the log record, which the
	// span id correlates.
	Statement bool
	// MaxSQLLength bounds the statement text on a span. It is the bound query
	// diagnostics already declares, so one setting bounds both surfaces.
	MaxSQLLength int
}

// TracePolicy returns the framework span policy of ctx, or nil when no
// framework span should be created. It is one context lookup, the same one
// every other resource accessor makes.
func TracePolicy(ctx context.Context) *Tracing { return resources(ctx).Trace }
