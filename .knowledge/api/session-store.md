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
  - Store[T].Touch(context.Context, keyHash, lastSeenAt, idleExpiresAt) error
  - Store[T].Delete(context.Context, keyHash) error
codec:
  scope: typed payload only
  reason: record timestamps stay in backend fields so renewal never rewrites the payload
  default: session.JSONCodec[T]
errors:
  - not found or expired
  - unavailable with sanitized backend detail
  - codec failure
rules:
  - honor cancellation and deadlines
  - never accept or return the raw cookie token
  - Put replaces one key atomically
  - Touch never revives an expired or missing record
  - Touch refuses a renewal past the absolute expiry
  - Delete is idempotent
  - returned typed data is treated as immutable
  - expiry stored by the backend is authoritative over anything the browser presents
adapters:
  - imported RDB plugin over database/sql
  - imported Redis-compatible plugin through requirement:contrib-redis-valkey
  - imported DynamoDB plugin through requirement:dynamodb-store
redis:
  backend_name: redis
  servers: Redis or Valkey
  compatibility: requirement:contrib-redis-valkey
  status: not implemented
rdb:
  backend_name: rdb
  session_plugin: popcornwave/plugin/session/rdb
  status: implemented
  constructor: rdb.NewStore[T](*sql.DB, session.Codec[T], Options)
  owned_table: popcornwave_session
  schema: MigrationSQL publishes the migration file; VerifySchema is the startup check
  schema_ownership: rule:framework-owned-tables
  dialect: SQLite DDL; other dialects are deferred
  expiry_sweep: Prune removes records that expire without being revoked
  driver_registration: separate database/sql driver import
  guaranteed_driver: requirement:contrib-sqlite
  in_memory: sqlite://:memory:
  future_schemes: require implemented and verified drivers before configuration acceptance
  shared_executor: session.rdb.source middleware uses the pool owned by api:rdb-middleware
dynamo:
  backend_name: dynamo
  session_plugin: popcornwave/plugin/session/dynamo
  status: designed, per requirement:dynamodb-session-store
  owned_table: popcornwave_session, created by requirement:dynamodb-migration rather than by a migration file
  schema: no migration file and no version table; the table definition is generated from the plugin's own tagged type
  expiry_sweep: none, per decision:dynamodb-session-expiry; a record is judged expired on read and removed by TTL
  atomic_touch: the renewal is one conditional UpdateItem, so no read-then-write window exists
  read_consistency: decision:dynamodb-session-read-consistency
  shared_client: the process client installed by api:dynamo-package; there is no dedicated form
plugins: decision:import-registered-session-plugins
extension: applications may supply another Store[T] without changing session middleware
```
