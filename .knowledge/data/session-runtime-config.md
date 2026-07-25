---
id: data:session-runtime-config
type: data
title: Session Runtime Config
---
The `[session]` binding selects login-session behavior, cookie policy, and storage through shared dotted key prefixes without separate backend tables.

```yaml
registration: automatically registered by pw
fields:
  enabled: bool
  backend: redis or rdb
  ttl: duration
  idle_timeout: optional duration
  renewal_interval: duration
  cookie.name: string
  cookie.path: string
  cookie.domain: optional string
  cookie.secure: bool
  cookie.http_only: bool
  cookie.same_site: strict, lax, or none
plugin_fields:
  plugin/session/redis:
    redis.dsn: redis or rediss URL
    redis.key_prefix: string
    redis.connect_timeout: duration
  plugin/session/rdb:
    rdb.source: middleware or dedicated
    rdb.dsn: dedicated-only URL such as sqlite://app.db or sqlite://:memory:
    rdb.table: string
    rdb.busy_timeout: duration
rules:
  - all keys are declared under one session binding
  - related fields share cookie, redis, or rdb prefixes
  - backend-specific keys exist only when decision:import-registered-session-plugins imports their plugin
  - validate only fields used by the selected imported backend
  - redis accepts Redis or Valkey endpoints through requirement:contrib-redis-valkey
  - middleware source reuses the *sql.DB owned by api:rdb-middleware and forbids session.rdb.dsn
  - dedicated source opens a separately owned pool and requires session.rdb.dsn
  - dedicated source delegates DSN handling to separately imported database/sql drivers
  - reject dedicated source when its canonical connection identity equals middleware.rdb.dsn; select middleware source instead
  - Popcorn Wave initially guarantees rdb with requirement:contrib-sqlite, including sqlite://:memory:
  - reject unimported backends and unregistered RDB drivers at startup
  - redact Redis and RDB DSN credentials and sensitive query values
contracts:
  store: api:session-store
  manager: api:session-manager
  plugin: api:session-backend-plugin
  lifecycle: flow:session-lifecycle
  security: policy:session-security
boundary: login sessions are distinct from single-use requirement:contrib-auth-state ceremony records
```
