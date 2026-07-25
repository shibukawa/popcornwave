---
id: requirement:modern-observability
type: requirement
title: Modern UI Observability
---
Tracing and metrics distinguish page, component, cache, data, action, streaming, and refresh work without exposing high-cardinality values.

```yaml
operations:
  - complete page request
  - server-component execution
  - cache hit, stale hit, miss, and revalidation
  - external function and database call
  - action execution and invalidation
  - streamed boundary completion
  - partial refresh and patch size
safe_dimensions:
  - component type ID
  - normalized route name
unsafe_dimensions:
  - instance key
  - raw component input
  - user value
```
