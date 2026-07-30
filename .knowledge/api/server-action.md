---
id: api:server-action
type: api
title: Server Action
---
A registered typed POST endpoint performs a component mutation and returns typed outcomes plus cache invalidation.

```yaml
rung:
  this: the typed contract of concept:modern-server-ui, which has no implementation yet
  delivered_instead: api:page-action-endpoint, whose handler is an ordinary one that owns its whole response
  difference: typed argument binding and tag-driven invalidation are what this rung adds; the endpoint address, the reachable surface, and the CSRF coverage are already the other one's
generation:
  - opaque action identifier
  - typed argument binding
security:
  - authentication and authorization
  - CSRF protection
  - request limits
application:
  transaction_boundary: explicit
results:
  - redirect
  - typed value
  - validation-driven patch
  - invalidation tags
post_success:
  - invalidate data and component caches
forbidden: invocation of unregistered functions
```
