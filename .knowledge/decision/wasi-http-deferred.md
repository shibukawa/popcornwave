---
id: decision:wasi-http-deferred
type: decision
title: Defer WASI HTTP Adapter
---
The first Petitweb release targets TinyGo native executables and defers component-model WASI HTTP hosting to a later adapter.

```yaml
status: accepted_for_mvp
mvp_target: TinyGo native executable using net/http
deferred:
  - WASI Preview 2 HTTP component adapter
  - runtime-specific WIT packaging
reason:
  - system:httpbinder public handlers use net/http types
  - WASI HTTP hosts require runtime-specific transport integration
future_boundary: keep handler business logic separable from listener startup
```
