---
id: requirement:shared-web-runtime
type: requirement
title: Shared Web Runtime
---
Both application styles use one lifecycle and HTTP foundation.

```yaml
capabilities:
  - api:application-lifecycle
  - data:loaded-configuration
  - decision:independent-runtime-config-bindings
  - policy:context-value-storage
  - api:request-context-accessors
  - requirement:typed-http-contract
  - policy:web-middleware
  - flow:request-lifecycle
  - policy:operational-endpoints
constraints:
  - startup validation precedes request acceptance
  - generated user code depends on Popcorn Web runtime, never the reverse
  - advanced features are application-level opt-ins
```
