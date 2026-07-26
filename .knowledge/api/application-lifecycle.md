---
id: api:application-lifecycle
type: api
title: Application Lifecycle API
---
The pw package can own the complete server lifecycle or return the same initialized middleware stack for an application-owned server.

```yaml
surface:
  - Run(context.Context, http.Handler, ...Option) error
  - Middlewares(http.Handler, ...Option) (http.Handler, error)
  - WithPublicFS(fs.FS) Option
  - middlewares.RegisterPublicFS(fs.FS)
run:
  - call api:runtime-configuration if configuration is not parsed
  - execute requirement:built-in-config-generation and return when its framework option is selected
  - initialize decision:config-driven-database from runtime configuration
  - initialize services and log safe values with provenance
  - initialize the framework middleware stack
  - mount api:authentication-endpoints when data:authentication-runtime-config is enabled
  - construct api:public-asset-middleware for the selected build mode
  - attach framework resources to request contexts
  - start the configured HTTP server
  - gracefully stop acceptance and drain requests where supported
  - close resources in reverse initialization order
middlewares:
  - performs the same configuration, service, and middleware initialization
  - returns the final standard http.Handler without starting a listener
platform:
  standard_go: signal.NotifyContext on os.Interrupt and SIGTERM drives graceful shutdown
  tinygo: no signal handler is installed; the caller's context is the only shutdown trigger
  rationale: TinyGo os/signal replaces the default disposition but delivers nothing, so registering would make the process unkillable by Ctrl+C or SIGTERM
  seam: notifyShutdownSignals, split by the tinygo build tag
rule: all startup validation failures occur before request acceptance
```
