---
id: requirement:contrib-auth-state
type: requirement
title: Authentication State Store
---
authstate provides expiring single-use state storage shared by requirement:contrib-passkey, requirement:contrib-oauth, and requirement:contrib-oidc.

```yaml
package: authstate
public_api:
  - Store[T] with Put(context, key, value, expiresAt) and atomic Take(context, key)
  - api:auth-state-codec for durable adapters
errors:
  - ErrCodec identifies bounded encode, malformed record, and decode failures
  - ErrUnavailable identifies sanitized Redis, Valkey, or SQLite availability failures
  - context cancellation remains detectable through context errors
adapters:
  - requirement:contrib-auth-state-memory
  - requirement:contrib-auth-state-redis
  - requirement:contrib-auth-state-sqlite
  - requirement:contrib-auth-state-dynamo
required:
  - Take removes a value atomically before returning it
  - expired values are removed and never returned
  - caller cancellation is honored
  - nil context is rejected with a stable error rather than panicking
  - stable errors reveal no stored value
constraints:
  - stored values are treated as immutable
  - implementations live in adapter subpackages; the contract package has no backend
  - applications select an adapter without changing authentication flows
```
