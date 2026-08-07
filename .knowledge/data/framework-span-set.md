---
id: data:framework-span-set
type: data
title: Framework Span Set
---
The spans Popcorn Wave opens inside a request root span, so a trace shows which branch a response took, how long the first paint held, and which statements it ran.

```yaml
configuration: data:observability-runtime-config observability.trace
root: the request server span of requirement:contrib-otel, opened by the tracing middleware whenever export exists or observability.trace.enabled is on
scope: github.com/shibukawa/popcornwave
render:
  name: "render <mode>"
  kind: internal
  modes:
    buffered: the whole document rendered before the first byte, which is what a bot or a disabled stream gets
    stream: flow:initial-streaming-render
    live: flow:live-boundary-delivery, the delivery response rather than the document
    navigate: the requirement:navigation-delta-rendering path of flow:partial-refresh
    redraw: the requirement:reloadable-component-endpoint path of flow:partial-refresh
    fragment: requirement:html-fragment-rendering
  attributes:
    pw.render.mode: the mode above, restated so a backend filters on it
    pw.render.layers: wrapper depth plus the leaf
    pw.render.async: the chain reports an await block
    pw.render.live: the chain reports a live block
    pw.render.bot: decision:bot-client-classification forced the buffered branch
    pw.render.bytes: bytes that reached the client, uncompressed; a live response counts its deliveries and not the document it discarded
    pw.render.boundaries: settled boundaries or deliveries, whatever observability.trace.boundary says
    pw.live.close_reason: done or retry, live only
  status: error when the render failed, including a stream that broke after commit and still answered 200
initial_build:
  name: render initial
  parent: the render span
  covers: the shell, the merged head, and every fallback
  boundary_signal: htmlbind flushes once when that pass ends, and the framework's response writer turns that flush into the end of this span
  omitted_on: the buffered and fragment modes, where the whole render is the initial build
  attributes:
    pw.render.bytes: the size of the committed document
await_boundary:
  name: render boundary
  parent: the render span
  extent: from the commit to the completion, which is how long that fallback held the screen rather than how long its work took
  why: a boundary is observed only on arrival, so the interval around it is the one thing measured rather than guessed
  attributes:
    pw.boundary.id: the positional placeholder id, which is generated and carries no user value
    pw.boundary.bytes: the settled fragment
live_delivery:
  name: live delivery
  parent: the live render span
  extent: from the previous delivery of the same boundary, or from the stream opening, so consecutive spans abut and read as one region changing
  attributes: the same two as an await boundary
database:
  name: the statement keyword, from a closed allowlist, falling back to the executor operation
  kind: client
  seam: decision:executor-seam-instrumentation, the same one data:query-record uses
  attributes:
    db.system.name: driver
    db.operation.name: the statement keyword
    db.query.text: bounded by observability.query.max_sql_length, and present only when observability.trace.statement is on
    pw.db.query_truncated: the text hit that bound
    pw.db.connection: the pool label, when more than one is configured
    pw.db.tx_depth: inside a transaction only
    pw.db.rows_affected: an exec only
    pw.db.slow: over data:query-diagnostics-config slow_threshold
  status: error with an exception event when the statement failed
  excluded:
    - bind values, per policy:query-log-safety
    - the EXPLAIN plan and the rerun snippet, which stay on data:query-record
    reason: a trace is retained longer and read more widely than a log, and the span id correlates the record that carries them
cost:
  disabled: one nil comparison per response and per statement; no span object, no wrapped writer, and no wrapped executor
  boundary_off: the render span still counts boundaries, because the count is an attribute rather than a span
cardinality:
  safe: mode, driver, statement keyword, generated boundary id
  absent: instance key, component input, user value, matching requirement:modern-observability
```
