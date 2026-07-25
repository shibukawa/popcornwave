---
id: api:session-store
type: api
title: Session Store Contract
---
Session backends persist typed data:session-record values behind one context-aware contract.

```yaml
surface:
  - Store[T].Put(context.Context, keyHash, Record[T]) error
  - Store[T].Get(context.Context, keyHash) returns Record[T]
  - Store[T].Touch(context.Context, keyHash, lastSeenAt, expiresAt) error
  - Store[T].Delete(context.Context, keyHash) error
errors:
  - not found or expired
  - unavailable with sanitized backend detail
  - codec failure
rules:
  - honor cancellation and deadlines
  - never accept or return the raw cookie token
  - Put replaces one key atomically
  - Touch never revives an expired or missing record
  - Delete is idempotent
  - returned typed data is treated as immutable
adapters:
  - imported Redis-compatible plugin through requirement:contrib-redis-valkey
  - imported RDB plugin over database/sql
redis:
  backend_name: redis
  servers: Redis or Valkey
  compatibility: requirement:contrib-redis-valkey
rdb:
  backend_name: rdb
  session_plugin: popcornwave/plugin/session/rdb
  driver_registration: separate database/sql driver import
  guaranteed_driver: requirement:contrib-sqlite
  in_memory: sqlite://:memory:
  future_schemes: require implemented and verified drivers before configuration acceptance
  shared_executor: session.rdb.source middleware uses the request database or active transaction
plugins: decision:import-registered-session-plugins
extension: applications may supply another Store[T] without changing session middleware
```
