---
id: api:session-backend-plugin
type: api
title: Session Backend Plugin Contract
---
An imported plugin registers one session backend factory and its generated configbind target before configuration load.

```yaml
registration:
  backend: unique name and Store factory
  config: generated type and prefix under session
  schema: optional host-side versioned schema provider
rdb_plugin:
  import: popcornwave/plugin/session/rdb
  backend: rdb
  config_prefix: session.rdb
  factory: reuses api:rdb-middleware or opens a dedicated database/sql-backed api:session-store
  driver_selection: DSN scheme resolves a separately imported database/sql driver
  schema_provider: deterministic dialect SQL, check, and apply for api:cli-session-schema
redis_plugin:
  import: popcornwave/plugin/session/redis
  backend: redis
  config_prefix: session.redis
  compatibility: requirement:contrib-redis-valkey
rules:
  - registration completes before configbind load
  - reject duplicate backend names and config identities
  - construct only the selected backend
  - reject configuration for an unimported plugin
  - reject an RDB DSN whose database/sql driver is not registered
  - middleware source requires enabled api:rdb-middleware
  - middleware source resolves api:request-context-accessors active SQL executor for every store operation
  - middleware source joins an active request transaction and never acquires a second pool connection for that operation
  - compare driver-provided canonical connection identities before opening a dedicated pool
  - never close the shared middleware database; close a dedicated session database during application shutdown
  - plugin initialization performs registration only and opens no connection
  - schema provider owns only plugin tables and schema-version metadata
```
