---
id: requirement:contrib-auth-state-redis
type: requirement
title: Redis Authentication State Store
---
contrib/authstate/redis implements requirement:contrib-auth-state over Redis or Valkey without implementing a Redis protocol client.

```yaml
package: contrib/authstate/redis
dependency: requirement:contrib-redis-valkey
transport: policy:outbound-transport-security
public_api:
  - NewStore[T](go-redis UniversalClient, api:auth-state-codec, Options)
  - Store[T] implements authstate.Store[T]
options:
  - required bounded key prefix and namespace
  - injectable clock
  - maximum key and encoded payload bytes with hard caps
record: data:auth-state-record
put:
  - encode and bound payload before network mutation
  - store version, absolute expiry, and payload with SET NX and a ceil-rounded millisecond TTL
  - false NX result maps to ErrAlreadyExists
take:
  - GETDEL atomically consumes one value
  - missing value maps to ErrNotFound
  - validate record bounds and absolute expiry before decode
  - malformed, expired, or undecodable values remain consumed
errors:
  - semantic failures map to requirement:contrib-auth-state errors
  - sanitized client and server failures wrap ErrUnavailable
operations:
  required:
    - SET with NX and PX
    - GETDEL
  excluded:
    - transactions, Pub/Sub, Streams, and Redis Cluster redirection logic in the adapter
security:
  - use decision:local-tls-proxy-boundary outside loopback or the same Pod namespace
  - retain Redis ACL or password authentication on the protected local hop when configured
  - never include Redis URLs, keys, or payloads in stable errors
```
