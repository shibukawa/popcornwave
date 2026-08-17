---
id: requirement:framework-metrics
type: requirement
title: Framework Metrics
---
requirement:contrib-otel gains a metric API and OTLP metric export, and the framework records the counters and histograms of data:framework-metric-set on the seams data:framework-span-set already measures, so a rate, a percentile, and a hit rate are read from every request rather than from the traces that survived requirement:trace-head-sampling.

```yaml
status: built 2026-08-18; the http, db, render, cache, and runtime groups all record, and the render group's live and boundary instruments record on the net/http half only
priority: should
source: user request 2026-08-17, naming request count, query count, request duration, query duration, requests over time, and cache hit rate as the shape wanted
holds: the gate, the reader, the cardinality rule, and the division between what OpenTelemetry already names and what only this framework can see
inventory: data:framework-metric-set
collection: flow:metric-collection
why_a_second_signal:
  decision: decision:metrics-are-not-sampled
  short: a span is a record of one request and is sampled, retained briefly, and read by a human; a metric is an aggregate of every request and is retained long and read by a query
  cost_shape: on the direct route of flow:telemetry-export a span costs per span, so traffic and bill move together; an instrument costs one export per interval whatever the traffic is, which is why the signal that survives decision:sampling-default-follows-the-environment is also the cheap one
  already_visible_in_this_catalog: requirement:modern-observability has said "tracing and metrics" since it was written and has listed only spans, and requirement:data-result-cache records four counters no signal carries out of the process
no_new_measurement:
  rule: every instrument reads a value the framework already computes, on the seam that already computes it
  examples: the request span of requirement:contrib-otel ends with a duration and a status; decision:executor-seam-instrumentation times every statement; the tally of requirement:component-output-cache is bound per response; pwruntime CacheStore.Stats already holds hits, misses, stale hits, and coalesced waits
  consequence: the cost of this feature is the aggregation, not a second stopwatch, and a value that disagrees with the span tree is a bug rather than a difference of method
  contrast: requirement:dev-request-timing-surface reads spans because it reports one request; a metric cannot read spans, because the sampled ones are the only spans there are
two_layers:
  semconv:
    rule: an instrument OpenTelemetry already specifies keeps its name, unit, and attribute set unchanged
    why: these are what a dashboard, an alert, and an operator's previous job already know; renaming them would make this framework the only reason a query has to be rewritten
    covers: http server and client, database client, and the go runtime — the same set a zero-code agent installs, which is what makes it the floor rather than the feature
  framework:
    rule: an instrument under the pw namespace, one per question no external agent can answer
    why_they_are_ours: a render mode, an await boundary, a live delivery, and a component cache hit are framework concepts; nothing outside the process can see them, and the six modes of data:framework-span-set are the closed set that makes them safe to aggregate
    the_valuable_half: decision:automatic-async-render-selection picks a branch per request, and no other signal reports how often it picked which, or what each branch cost
not_separate_instruments:
  purpose: the questions asked of this requirement that are already answered, so no instrument is added for them
  request_count: the count of the http.server.request.duration histogram; a counter beside it would be a second name for one number
  requests_over_time: a rate over that count, computed by whatever reads the metric; delta temporality of flow:metric-collection is what lets a reader as simple as the requirement:dev-telemetry-viewer pane chart it without differencing
  average_duration: the histogram sum over its count
  percentile: the histogram buckets, which is the reason duration is a histogram and not a gauge of the last value
  query_count: the count of db.client.operation.duration, per driver and statement keyword
  cache_hit_rate: hits over hits plus misses, from the two counters requirement:component-output-cache already reports together and for the reason it records; a ratio computed in the process would lose the denominator that makes it readable
  rule: emit the terms, never the quotient, and never a pre-windowed count
cardinality:
  safe: the safe_dimensions of requirement:modern-observability, minus one — a boundary id is positional but unbounded across pages, so it stays a span attribute and reaches no metric
  route:
    needed: http.route, without which every server metric is one series for the whole application
    obstacle: net/http sets Pattern on the request copy the mux hands the handler, and the frame that records the metric wraps the mux and never sees that copy
    resolved_differently_per_transport:
      net_http: the pattern travels back up on the middlewares ResponseTracker, which is the one value the handler and the frame both hold; pw.SetRoute writes it, and every framework response writer calls that, so a generated page needs nothing and a hand-written handler makes one call
      fasthttp: the mux is this framework's own type there, so it records the pattern at dispatch and no handler is asked; pwfast.SetRoute exists for the rewritten call to land on and has nothing left to do
      why_not_one_mechanism: api:serve-mux is a type alias to the standard library on one half and a framework type on the other, so the earlier seam is available on exactly one of them
    absent_rather_than_raw: a request that matched no route reports no route attribute, and never the raw path, which is the unbounded value the attribute exists to avoid
  forbidden: a path value, a bind value, a subject, an instance key, a component input, a trace id, an error message
  error_type: error.type carries a class from a closed set, never a message
gate:
  form: an observability.metrics section of data:observability-runtime-config, beside trace and under the same otel parent
  enabled: auto, on, or off, resolving the way trace.enabled does — auto follows whether an exporter exists, because an aggregation nothing exports is pure cost
  granularity: one key per group of data:framework-metric-set, so a deployment can keep the http and db groups and decline the runtime group
  off: no reader, no instrument, no observable registration, and one nil comparison on each recording path, matching the cost rule of data:framework-span-set
  independent_of_trace: metrics on with tracing off is a supported configuration and the one a heavily sampled deployment ends up in
parity:
  rule: requirement:second-build-feature-parity applies; the pwfast frames record the same instruments with the same attributes
  buffered_only: api:pwfast-package renders into a buffer, so its render metrics carry the buffered mode and no boundary or live series, which is a shape difference rather than a gap
dev_loop:
  receiver: requirement:dev-telemetry-viewer already accepts /v1/metrics and already renders gauges, sums, histograms, and summaries; this is what fills the pane it currently leaves empty
  default: the injected endpoint turns metrics auto on with traces, so a developer who configured nothing sees them
  process_overlap:
    fact: the viewer samples cpu, memory, threads, open files, and io of the process from outside it
    kept_separate: the go runtime group reports heap, goroutines, and gc from inside the process, which the sampler cannot see, and neither is derived from the other
    non_goal: re-exporting the viewer's process samples as metrics, which would make the same numbers arrive twice by two paths that disagree at every interval
acceptance:
  - a request served by either transport increments one series of http.server.request.duration keyed by method, status, and route
  - the count of that histogram over an interval answers requests over time without a second instrument
  - a page that ran statements reports db.client.operation.duration per driver and statement keyword, and the count matches the client spans in the same trace
  - a response that consulted the component cache reports a hit and a miss counter, and their ratio matches the pw.render.cache_hits and pw.render.cache_misses of that response's render span
  - a data cache configured under data:cache-store-set reports the four counters CacheStore.Stats already holds, closing the not_built line of requirement:data-result-cache
  - each render mode of data:framework-span-set is a distinct series of pw.render.duration, and no series carries a path or a parameter
  - metrics off starts no reader and registers no observable
  - metrics on with observability.trace off still counts every request, which is the configuration requirement:trace-head-sampling makes ordinary
  - no metric attribute carries a value policy:query-log-safety keeps off a span
  - a metric series survives a request whose trace was not sampled
non_goals:
  - a metric derived from a span, per decision:metrics-are-not-sampled
  - a ratio, an average, or a per-minute count computed inside the process
  - exemplars, which link a metric bucket back to a trace and would reintroduce the sampling dependency this requirement exists to remove
  - views, custom aggregations, or a configurable bucket boundary set in the first version
  - a prometheus scrape endpoint; export is OTLP only, and a deployment wanting scrape puts a collector in front, the same shape policy:layered-cache uses for the http cache layer
  - metrics for the http cache layer, which policy:layered-cache puts in front of the process
  - instrumenting an application's own domain counters, which reach the metric API directly the way a handler already reaches the trace API
  - a metric about the metric pipeline; the dropped-record count of flow:telemetry-export stays a log and a viewer field, because a pipeline reporting its own loss through itself reports nothing when it fails
open:
  route_seam: where the matched pattern is carried from the mux to the recording frame — a cell on the middlewares ResponseTracker, a context value, or the page registry that already knows every pattern
  bucket_boundaries: the semconv defaults are chosen for seconds of network latency, and a render or a statement on a loopback dev run occupies the first bucket
  db_pool_observables: whether the connection group label is always present or only when more than one pool is configured, which data:framework-span-set decided the other way for spans
  temporality_default:
    settled: it is a key, observability.metrics.temporality, defaulting to delta because the dev viewer is the first reader and charts it without differencing
    still_open: whether a long-lived deployment on the direct route should default the other way, which the loss section of flow:metric-collection is the argument for
  observable_temporality: an observable sum is always exported cumulative whatever the key says, because the callback answers a running total and labelling that a delta would claim one interval's worth; a gauge carries no temporality at all
```
