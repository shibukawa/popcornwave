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
  results: synchronous or asynchronous
  cache: declarative policy and invalidation tags
forbidden:
  - reflection-based dispatch
  - arbitrary dynamic function lookup
```
