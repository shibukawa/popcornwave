---
id: requirement:modern-observability
type: requirement
title: Modern UI Observability
---
Tracing and metrics distinguish page, component, cache, data, action, streaming, and refresh work without exposing high-cardinality values.

```yaml
spans: data:framework-span-set
metrics: data:framework-metric-set, specified by requirement:framework-metrics; the operations below name what a span reports about one request, and an instrument reports how often and how much over all of them
separation: decision:metrics-are-not-sampled, which is why no operation below is aggregated into a metric and why requirement:trace-head-sampling changes no count
configuration: data:observability-runtime-config observability.trace and observability.metrics
operations:
  complete page request: the request server span, opened by the tracing middleware
  server-component execution: the render span, one per response, naming which of the six branches it took
  initial build: the child span covering the shell, the merged head, and every fallback, ended by the flush that commits the document
  streamed boundary completion: one span per settled await boundary, spanning the commit to the completion
  live delivery: one span per delivery, spanning the previous delivery of that boundary
  external function and database call: a client span per executed statement, on the same seam data:query-record uses
  partial refresh and patch size: the navigate and redraw render modes, with pw.render.bytes
  component cache hit and miss: both on the render span, as pw.render.cache_hits and pw.render.cache_misses of data:framework-span-set, reported together and only by a response that consulted the store of requirement:component-output-cache
  data cache hit, stale hit, miss, coalesced wait, and revalidation: counted on the store of requirement:data-result-cache and readable only inside the process; the pw.data_cache instruments of data:framework-metric-set are what carry them out, and no span reports them because a detached revalidation has no request to belong to
  http cache hit and revalidation: not instrumented, and not the framework's to observe — policy:layered-cache puts that layer in front of the process
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
