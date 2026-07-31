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
  backend: rdb, cookie, or redis
  ttl: duration
  idle_timeout: optional duration
  renewal_interval: duration
  cookie.name: string
  cookie.path: string
  cookie.domain: optional string
  cookie.secure: bool
  cookie.http_only: bool
  cookie.same_site: strict, lax, or none
  cookie_store.name: cookie holding the sealed record under backend cookie
  cookie_store.secret: base64 secret of at least 256 bits, read from SESSION_COOKIE_STORE_SECRET or a ${NAME} reference
  cookie_store.previous_secrets: retired secrets kept readable during a rotation
plugin_fields:
  sessionstore/redis:
    redis.dsn: redis or rediss URL, read from SESSION_REDIS_DSN or a ${NAME} reference
    redis.key_prefix: string
    redis.connect_timeout: duration bounding the startup ping and per-command deadlines
  sessionstore/<engine>:
    rdb.source: middleware or dedicated
    rdb.group: middleware-only data:database-connection-set group holding the session tables
    rdb.dsn: dedicated-only URL such as sqlite://app.db or sqlite://:memory:
    rdb.table: string
    rdb.busy_timeout: duration
implemented:
  binding: enabled, backend, ttl, idle_timeout, renewal_interval, and every cookie and cookie_store key
  rdb_keys: rdb.source, rdb.dsn, and rdb.table are declared by pw rather than by the plugin
  backend: rdb, cookie, and redis
  redis_keys: redis.dsn, redis.key_prefix, and redis.connect_timeout are declared by pw rather than by the plugin
  source: middleware only
deferred:
  - dedicated rdb source and rdb.busy_timeout
  - plugin-owned registration of backend-specific keys
rules:
  - all keys are declared under one session binding
  - related fields share cookie, cookie_store, redis, or rdb prefixes
  - the cookie backend needs no storage and reuses the cookie policy for its record cookie
  - reject an empty or under-length cookie_store.secret when backend is cookie, per decision:cookie-session-storage
  - reject an empty redis.dsn, a non-redis scheme, or a server that fails the startup ping when backend is redis
  - report a malformed redis.dsn by shape only, because the URL can carry a password
  - keep the sealing secret out of the file itself; the error naming a bad secret never repeats it
  - a backend other than cookie requires the blank import that registers it, per decision:import-registered-session-plugins
  - a selected backend with no registered factory fails startup with the missing import line named
  - validate only fields used by the selected imported backend
  - redis accepts Redis or Valkey endpoints through requirement:contrib-redis-valkey
  - middleware source reuses a *sql.DB owned by api:rdb-middleware and forbids session.rdb.dsn
  - middleware source resolves its group through policy:connection-group-selection and rejects a readonly one
  - dedicated source opens a separately owned pool and requires session.rdb.dsn
  - dedicated source delegates DSN handling to separately imported database/sql drivers
  - reject dedicated source when its canonical connection identity equals middleware.rdb.dsn; select middleware source instead
  - Popcorn Wave initially guarantees rdb with requirement:contrib-sqlite, including sqlite://:memory:
  - reject unimported backends and unregistered RDB drivers at startup
  - redact Redis and RDB DSN credentials and sensitive query values
contracts:
  tiers: requirement:state-storage-tiers
  store: api:session-store
  manager: api:session-manager
  plugin: api:session-backend-plugin
  lifecycle: flow:session-lifecycle
  security: policy:session-security
boundary: login sessions are distinct from single-use requirement:contrib-auth-state ceremony records
```
