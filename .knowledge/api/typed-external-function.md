---
id: api:typed-external-function
type: api
title: Typed External Function
---
Templates call only declared functions bound by generated code to ordinary Go functions.

```yaml
generation_validation:
  - name
  - arguments
  - result type
  - error behavior
runtime:
  context: explicit request context and cancellation
  results: synchronous, or `external async` returning a value and an error
  async_placement: an async call is legal only inside an await binding, and its work starts when the boundary reaches it
  caller_started_alternative: api:async-html-value starts the work earlier
  cache: declarative policy and invalidation tags
forbidden:
  - reflection-based dispatch
  - arbitrary dynamic function lookup
```
