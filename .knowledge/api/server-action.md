---
id: api:server-action
type: api
title: Server Action
---
A registered typed POST endpoint performs a component mutation and returns typed outcomes plus cache invalidation.

```yaml
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
