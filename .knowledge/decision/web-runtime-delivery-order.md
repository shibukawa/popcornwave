---
id: decision:web-runtime-delivery-order
type: decision
title: Web Runtime Delivery Order
---
Deliver reusable lower layers before dependent modern UI features.

```yaml
order:
  - requirement:shared-web-runtime and requirement:classic-rendering
  - api:typed-external-function and concept:modern-server-ui
  - memory-backed policy:layered-cache with TTL, private scope, and tags
  - flow:initial-streaming-render
  - flow:partial-refresh
  - api:server-action with automatic invalidation
  - concept:client-component with selective module loading
  - distributed caches and stale-while-revalidate
first_modern_release_non_goals:
  - React Server Components compatibility
  - browser virtual DOM
  - full-page hydration
  - WebSocket navigation
  - browser execution of server-component code
  - distributed component scheduling
  - inference of every database dependency
```
