---
id: requirement:modern-web-acceptance
type: requirement
title: Modern Web Acceptance
---
A modern build delivers meaningful server HTML, incremental patches, safe default caching, and optional isolated client behavior on the shared runtime.

```yaml
criteria:
  - meaningful HTML precedes client JavaScript
  - flow:initial-streaming-render resolves independent async boundaries
  - requirement:async-html-rendering keeps the handler surface unchanged when a page starts streaming
  - flow:partial-refresh updates only affected boundaries
  - unchanged output hashes avoid fragment transfer
  - policy:layered-cache caches safe queries and deterministic components by default
  - api:server-action invalidates data and component caches by explicit tags
  - concept:client-component does not require full-page hydration
  - normal navigation, actions, and refresh require neither WebSocket nor SSE
  - standard handlers and middleware remain usable below the component runtime
security: policy:server-ui-security
operations: requirement:modern-observability
```
