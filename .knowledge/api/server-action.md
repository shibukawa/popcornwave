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
  mechanism_found_2026_08_13:
    what_was_missing: not the machinery but a key, since an arbitrary signature carries nothing that says it is an action and a returned value carries nothing that says which response it becomes
    the_two_answers: decision:typed-action-declaration admits it, and decision:typed-action-is-call-only decides the response by narrowing the caller
    taken_as: requirement:typed-server-action, which is this rung's argument binding half
    what_it_leaves_here: the invalidation tags and the cache invalidation below, which are a separate question from how a function is reached
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
