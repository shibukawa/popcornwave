---
id: requirement:modern-observability
type: requirement
title: Modern UI Observability
---
Tracing and metrics distinguish page, component, cache, data, action, streaming, and refresh work without exposing high-cardinality values.

```yaml
spans: data:framework-span-set
configuration: data:observability-runtime-config observability.trace
operations:
  complete page request: the request server span, opened by the tracing middleware
  server-component execution: the render span, one per response, naming which of the six branches it took
  initial build: the child span covering the shell, the merged head, and every fallback, ended by the flush that commits the document
  streamed boundary completion: one span per settled await boundary, spanning the commit to the completion
  live delivery: one span per delivery, spanning the previous delivery of that boundary
  external function and database call: a client span per executed statement, on the same seam data:query-record uses
  partial refresh and patch size: the navigate and redraw render modes, with pw.render.bytes
  cache hit, stale hit, miss, and revalidation: not instrumented; policy:layered-cache has no framework-owned store to observe yet
  action execution and invalidation: the handler's own work, which opens its own spans through the requirement:contrib-otel trace API
safe_dimensions:
  - component type ID
  - normalized route name
  - render mode, driver, and statement keyword, each from a closed set
  - generated boundary id, which is positional
unsafe_dimensions:
  - instance key
  - raw component input
  - user value
  - bind values, which policy:query-log-safety keeps on the correlated record and out of every span
cost:
  rule: a process that exports nothing opens no span, because auto reads the export switch
  disabled: one nil comparison per response and per statement
```
