---
id: data:session-runtime-config
type: data
title: Session Runtime Config
---
The `[session]` binding selects where per-browser state lives and how its token cookie travels, and declares no duration at all.

```yaml
registration: automatically registered by pw
declaration: the struct lives in popcornwave/sessionconfig and pw re-exports it as a true alias, per decision:framework-owned-session-extension
scope: placement and cookie policy; every lifetime is declared by data:authentication-runtime-config, per decision:session-lifetime-owned-by-auth
fields:
  enabled: bool
  backend: rdb, cookie, redis, or dynamo; which server backend a server-placed slot uses, never whether a slot is server-placed
  retention: how long the store may hold one record, default 720h, narrowed by the auth.session lifetime, per decision:storage-bounded-session-record
  cookie.name: string
  cookie.path: string
  cookie.domain: optional string
  cookie.secure: bool
  cookie.http_only: bool
  cookie.same_site: strict, lax, or none
  cookie_store.name: cookie holding the sealed record under backend cookie
  keyring.secret: base64 secret of at least 256 bits, read from SESSION_KEYRING_SECRET or a ${NAME} reference
  keyring.previous_secrets: retired secrets kept readable during a rotation
keyring:
  serves: session.ReadOnly signing and session.Private sealing alike, because session.Keyring derives one purpose-separated subkey per mode from one secret, per policy:cookie-value-protection
  required_when: any registered slot is not session.Shared, which is the only placement that protects nothing
  classification: secret, so policy:log-emission redaction and the rule:configuration-advisories literal-secret-in-config-file check both apply without naming the field
  renamed_from:
    keys: cookie_store.secret and cookie_store.previous_secrets
    reason: the prefix said the key belonged to the cookie backend, and decision:slot-declared-placement made it the session-wide keyring that a deployment on rdb or redis also needs
    kept: cookie_store.name, which really is the cookie backend's own record cookie
    transitional: a configuration still carrying cookie_store.secret fails startup naming keyring.secret, rather than being ignored
development_generation:
  problem: requiring an authored secret to run a scaffolded project puts a deployment concern in the way of getting started
  generation: api:cli-init generates a per-project keyring from crypto/rand and writes it as a literal into the scaffolded config.dev.toml
  persistence: on disk, so it survives restarting the developer loop, the machine, and the day
  rejected_startup_generation:
    shape: generating per process, or per developer loop, and discarding it at shutdown
    defect: every signed and sealed value dies with the process, so restarting the loop logs the developer out and empties every cart and preference being worked on
    note: the requirement:contrib-devidp client credentials are generated that way for good reason, because they mean nothing beyond one run; a keyring means the opposite
  scope: the dev token only; a token other than dev has no generation path and must supply the secret, which is the point
  cost:
    fact: config.dev.toml is normally committed, so one generated secret is shared by every clone of the project
    accepted: the value protects localhost development cookies, and rule:configuration-advisories already grades a dev literal as a note rather than a finding to act on
    override: a developer wanting a private one sets SESSION_KEYRING_SECRET, which data:loaded-configuration ranks above TOML
  guardrail: the same file diagnosed as any other token reports literal-secret-in-config-file as an error, so the dev convenience cannot travel to a deployment unnoticed
  not_a_placeholder: the value is random per project, so it is not what the rule:configuration-advisories scaffolded-or-placeholder-secret check looks for; that check still covers a fixed value shipped in a template
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
  sessionstore/dynamo:
    dynamo.table: declared table name, default popcornwave_session, resolved by rule:dynamodb-table-naming
    dynamo_has_no_endpoint: data:dynamodb-runtime-config already opens the client this backend borrows
    dynamo.consistent_read: bool, default false, per decision:dynamodb-session-read-consistency
moved_out:
  keys: ttl, idle_timeout, and renewal_interval
  to: the auth.session binding, whose struct lives in popcornwave/sessionconfig and which popcornwave/plugin/auth binds
  reason: decision:session-lifetime-owned-by-auth
  read_by: pw, which enforces the durations without importing the plugin that declares them
  transitional: a configuration still carrying session.ttl fails startup naming the auth.session key that replaced it, rather than being ignored
implemented:
  binding: enabled, backend, and every cookie, cookie_store, and keyring key; the durations are under auth.session
  rdb_keys: rdb.source, rdb.dsn, and rdb.table are declared by pw rather than by the plugin
  backend: rdb, cookie, and redis
  redis_keys: redis.dsn, redis.key_prefix, and redis.connect_timeout are declared by pw rather than by the plugin
  source: middleware only
deferred:
  - dedicated rdb source and rdb.busy_timeout
  - plugin-owned registration of backend-specific keys
rules:
  - all keys are declared under one session binding
  - retention is the one duration here, and it bounds the table rather than a proof; every other duration is a misplaced auth.session key
  - reject a non-positive retention for a server backend, because the sweep reads a zero expiry as already past and the renewal statement never matches one
  - related fields share cookie, cookie_store, keyring, redis, or rdb prefixes
  - the cookie backend needs no storage and reuses the cookie policy for its record cookie
  - the backend key names which server backend to use; api:session-registry decides which slots go there, per decision:slot-declared-placement
  - reject an empty or under-length keyring.secret once any slot other than session.Shared is registered
  - the requirement is not new with decision:slot-declared-placement; session.ReadOnly already needed it to sign, and that decision only added session.Private under a server backend
  - reject an unset keyring.secret whatever the token, because development_generation above wrote one into the dev file rather than leaving it unset
  - reject a keyring.secret literal in the TOML outside dev, which rule:configuration-advisories reports as literal-secret-in-config-file
  - reject backend cookie when any session.ServerOnly slot is registered, naming the slot, because that slot asked for revocation the cookie backend cannot give
  - reject an empty redis.dsn, a non-redis scheme, or a server that fails the startup ping when backend is redis
  - report a malformed redis.dsn by shape only, because the URL can carry a password
  - keep the keyring out of the file itself for every token but dev, where development_generation above deliberately writes one; the error naming a bad secret never repeats it
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
  - the dynamo backend carries no endpoint or credential of its own, because data:dynamodb-runtime-config already holds them
  - the dynamo backend rejects a configured ttl longer than what the deployment can expire, since decision:dynamodb-session-expiry leaves removal to TTL
  - redact Redis and RDB DSN credentials and sensitive query values
contracts:
  tiers: requirement:state-storage-tiers
  registry: api:session-registry
  store: api:session-store
  manager: api:session-manager
  plugin: api:session-backend-plugin
  lifecycle: flow:session-lifecycle
  security: policy:session-security
boundary:
  - session storage is distinct from login, per concept:session-storage-boundary
  - login sessions are distinct from single-use requirement:contrib-auth-state ceremony records
```
