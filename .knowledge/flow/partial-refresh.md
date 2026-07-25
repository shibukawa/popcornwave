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
client:
  - ignore or cancel superseded requests
  - reject stale rollback by sequence
  - drop child patches replaced by an enclosing parent
  - preserve focus, selection, and scroll where possible
```
