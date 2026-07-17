---
id: requirement:contrib-redis-valkey
type: requirement
title: TinyGo Redis and Valkey Client
---
Petitweb supports a bounded Redis-compatible subset for shared sessions and lightweight key-value state against Redis and Valkey.

```yaml
package: contrib/redis
support_tier: first_class_through_local_proxy
transport: policy:outbound-transport-security
protocol: RESP2
servers:
  - Redis
  - Valkey
use_cases:
  - expiring session records
  - replay and one-time token state
  - counters and rate-limit state
  - small shared key-value records
required_commands:
  - PING
  - GET
  - SET with expiry and NX or XX
  - DEL and GETDEL
  - EXPIRE and TTL
  - INCR
  - bounded EVAL for compare-and-delete
topologies:
  first_class:
    - standalone server
    - Sentinel-selected writable endpoint exposed through the local proxy
    - managed cluster hidden behind a stable proxy endpoint
  unsupported:
    - direct Redis Cluster MOVED or ASK redirection through a single TCP TLS tunnel
client_evidence:
  verified: 2026-07-17
  tinygo: 0.41.1
  passing:
    - bounded direct RESP2 interoperability with Redis 8.4.4 and Valkey 9.1.0
    - go-redis v9.17.3 compiled and passed session operations against both servers
  incompatible_unchanged:
    - go-redis v9.21.0 requires unavailable net.DefaultResolver behavior
    - valkey-go v1.0.76 requires unavailable TLS and TCP APIs
boundaries:
  - no Pub/Sub, Streams, modules, administration, or arbitrary unbounded replies in the initial subset
  - direct TinyGo TLS remains experimental under decision:local-tls-proxy-boundary
  - credentials and session values never enter logs or stable error text
```
