---
id: policy:web-middleware
type: policy
title: Web Middleware Integration
---
Infrastructure composes through standard http.Handler middleware, with framework-specific forms adaptable to common Go signatures where feasible.

```yaml
concerns:
  - request IDs and structured logging
  - panic recovery
  - authentication, authorization, sessions, and CSRF
  - CORS and compression
  - request size, timeout, and rate limits
  - caching
  - metrics and minimal OpenTelemetry propagation and export
  - trusted proxy and forwarded-header handling
ordering:
  - canonicalize and reject malformed request targets
  - apply policy:security-response-headers
  - apply api:rdb-middleware and selected transaction policy
  - load api:session-manager state
  - establish data:request-authentication
  - apply policy:authenticated-path-protection
  - apply policy:csrf-protection
  - run application authorization and handler
configuration:
  - data:middleware-runtime-config controls the enabled framework set and effective order
  - startup validation rejects missing dependencies and unsafe ordering
  - applications may wrap the final handler with additional standard middleware
application:
  - api:application-lifecycle Run applies the configured stack
  - api:application-lifecycle Middlewares returns the same wrapped handler
portability:
  framework_set: the concerns listed above are the set decision:backend-specific-middleware ports in full, one implementation per backend behind one build-tagged name
  application_middleware: backend-specific source with no portability promise, and the wrapping seam stays legal under decision:transport-handle-containment
  unchanged_here: the configured order, data:middleware-runtime-config, and startup validation, which describe the stack rather than its implementation
```
