---
id: flow:request-lifecycle
type: flow
title: Typed Request Lifecycle
---
A Popcorn Wave request remains a standard net/http exchange enriched with initialized framework resources and compact pw binding and response calls.

```yaml
flow:
  trigger: ServeMux-compatible router dispatches a route from decision:stdlib-servemux
  steps:
    - id: context
      action: framework middleware attaches config, logger, database, session, tracing, and security resources
    - id: bind
      actor: api:request-binding
      action: pw.Parse maps request sources into the typed input
      failure: write policy:validation-errors through api:problem-response and stop
    - id: validate
      action: application evaluates business constraints using policy:validation-errors
      failure: negotiate api:problem-response output and stop
    - id: execute
      action: application runs business logic
    - id: write
      action: api:html-response, api:api-response, or api:typed-stream writes the response
      failure: write a safe internal problem when headers are not committed
```
