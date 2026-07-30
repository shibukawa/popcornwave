---
id: flow:partial-refresh
type: flow
title: Boundary Partial Refresh
---
An interaction refreshes only affected addressable boundaries through one patch endpoint.

```yaml
endpoint: POST /_pw/refresh
request:
  - typed event or form snapshot
  - displayed boundary versions
  - request sequence number
server:
  - verify data:component-boundary
  - derive affected boundaries from data:ui-dependency-graph
  - return changed HTML patches only
  - frame each patch as boundary id plus HTML through the api:html-boundary-protocol fetch envelope, never as marker markup
client:
  - rewrite inserted placeholder ids into a per-response namespace, per api:html-boundary-protocol
  - apply by namespaced boundary id through the same runtime core the parser path uses
  - ignore or cancel superseded requests, whose completions then find no placeholder and no-op
  - reject stale rollback by sequence
  - drop child patches replaced by an enclosing parent
  - preserve focus, selection, and scroll where possible
unmanaged_alternative: requirement:html-fragment-rendering, where the application owns the route and an external swap library owns application, with no envelope and no boundary identity
```
