---
id: concept:classic-web-style
type: concept
title: Classic Web Style
---
An HTTP handler binds one request, runs application logic, and writes one complete response using standard navigation and form semantics.

```yaml
mode_name: classic
unit: http.Handler
rendering: requirement:classic-rendering
navigation:
  links: full document requests
  forms: ordinary submissions and redirects
  enhancement: optional
mutation:
  location: handler or application service
  browser_default: Post/Redirect/Get
  transaction_boundary: automatic request transaction by default or explicit api:transaction-runner when middleware.rdb.auto_transaction is false
caching:
  - HTTP validators and response cache
  - safe generated read-query cache
  - application cache
```
