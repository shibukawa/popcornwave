---
id: flow:request-lifecycle
type: flow
title: Typed Request Lifecycle
---
A Petitweb request remains a standard net/http exchange whose mapping and error payloads are handled by httpbind-go.

```yaml
flow:
  trigger: net/http ServeMux dispatches a route from decision:stdlib-servemux
  steps:
    - id: bind
      actor: system:httpbinder
      action: map request sources into the typed request
      failure: write policy:validation-errors and stop
    - id: validate
      action: application evaluates business constraints using policy:validation-errors
      failure: write problem details and stop
    - id: execute
      action: application runs business logic
    - id: write
      actor: system:httpbinder
      action: serialize typed response using Accept negotiation
      failure: write a safe internal problem when headers are not committed
```
