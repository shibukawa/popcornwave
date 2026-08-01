---
id: requirement:contrib-auth-state-memory
type: requirement
title: In-Memory Authentication State Adapter
---
authstate/memory provides bounded process-local requirement:contrib-auth-state storage.

```yaml
package: authstate/memory
public_api:
  - NewStore[T](Options)
required:
  - injected clock supports deterministic expiry tests
  - concurrent Put and Take are race safe
  - expired entries are reclaimed before capacity checks
  - nil context and nil or zero Store return stable errors without panic
  - duplicate keys fail without replacing stored values
limits:
  default_entries: 4096
  hard_entries: 65536
  default_key_bytes: 256
  hard_key_bytes: 4096
constraints:
  - process-local; intended for development, tests, and single-process deployments
  - values are stored by assignment and treated as immutable
```
