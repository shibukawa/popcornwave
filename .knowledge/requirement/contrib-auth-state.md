---
id: requirement:contrib-auth-state
type: requirement
title: Authentication State Store
---
contrib/authstate provides expiring single-use state storage shared by requirement:contrib-passkey, requirement:contrib-oauth, and requirement:contrib-oidc.

```yaml
package: contrib/authstate
public_api:
  - Store[T] with Put(context, key, value, expiresAt) and atomic Take(context, key)
  - NewMemoryStore[T](options)
required:
  - Take removes a value atomically before returning it
  - expired values are removed and never returned
  - caller cancellation is honored
  - nil context is rejected with a stable error rather than panicking
  - injectable clock supports deterministic expiry tests
  - concurrent Put and Take are race safe
  - stable errors reveal no stored value
  - configured entry and key limits have hard upper bounds
constraints:
  - stored values are treated as immutable
  - memory storage is process-local and intended for development, tests, and single-process deployments
  - production applications may replace Store without changing authentication flows
  - `MemoryStore` hard limits are 65536 entries and 4096 bytes per key
```
