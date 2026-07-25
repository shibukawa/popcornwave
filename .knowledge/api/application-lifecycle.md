---
id: api:application-lifecycle
type: api
title: Application Lifecycle API
---
The pw package can own the complete server lifecycle or return the same initialized middleware stack for an application-owned server.

```yaml
surface:
  - Run(context.Context, http.Handler) error
  - Middlewares(http.Handler) (http.Handler, error)
run:
  - call api:runtime-configuration if configuration is not parsed
  - initialize services and log safe values with provenance
  - initialize the framework middleware stack
  - attach framework resources to request contexts
  - start the configured HTTP server
  - gracefully stop acceptance and drain requests where supported
  - close resources in reverse initialization order
middlewares:
  - performs the same configuration, service, and middleware initialization
  - returns the final standard http.Handler without starting a listener
platform:
  standard_go: signal.NotifyContext may drive graceful shutdown
  tinygo: signal-driven shutdown may be omitted when unsupported
rule: all startup validation failures occur before request acceptance
```
