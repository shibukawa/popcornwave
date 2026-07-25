---
id: policy:server-ui-security
type: policy
title: Server UI Security and Correctness
---
Generated endpoints, streamed instructions, output contexts, and asynchronous work remain server-validated and bounded.

```yaml
rules:
  - escape HTML, attribute, URL, JavaScript, JSON, SQL, and other contexts independently
  - make raw output explicit and reviewable
  - validate every action and refresh request on the server
  - bind data:component-boundary to route, scope, and build version
  - protect api:server-action with CSRF defenses
  - use CSP nonce or hash for streamed inline instructions
  - hide server props, configuration, SQL, cache keys, and credentials
  - limit body size, component depth, execution time, and async concurrency
  - propagate cancellation to queries, external calls, and async components
  - order patches deterministically and reject stale build versions
```
