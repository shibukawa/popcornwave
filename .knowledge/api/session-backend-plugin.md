---
id: api:session-backend-plugin
type: api
title: Session Backend Plugin Contract
---
A session backend is selected by name at startup and contributed by a blank import, so an application links only the storage it configured.

```yaml
registration:
  owner: pw, which already declares data:session-runtime-config
  call: pw.RegisterSessionBackend(name, factory) from plugin init
  uniqueness: a duplicate backend name is a registration panic, not a silent replacement
  timing: init runs before configbind load, so every import is registered before selection
factory:
  signature: func(context.Context, pw.SessionConfig, pw.SessionResources) (session.Backend, error)
  typing: the factory is not generic, which is what lets one registry hold every backend
  duties:
    - read only the keys of its own backend prefix
    - open and validate its own dependencies before returning
    - name the failing key when its own configuration is unusable
backend_value:
  store: session.RawStore, the non-generic contract over already encoded payloads
  close: optional; releases a client the backend opened, never a resource the host lent it
  prune: optional; the expiry sweep of a backend that accumulates records
  rule: a host reads capabilities from the returned value and never type-asserts a plugin type
typing_seam:
  host: the api:session-registry encodes every session.Private slot into the one payload a RawStore takes
  payload: the codec belongs to the host, so a backend never sees an application payload type
  implemented: session.Typed[T](RawStore, Codec[T]) returns the Store[T] a generic manager takes, which the registry replaces
  request_binding: Typed forwards the RequestBinder of a store that keeps records in the browser
built_in:
  cookie:
    registered_by: pw itself, because decision:cookie-session-storage needs no dependency
    import: none
imported:
  rdb:
    import: _ "popcornwave/sessionstore/<engine>", where the engine is the dialect the DSN resolves to
    config_prefix: session.rdb
    resources: reuses the *sql.DB of api:rdb-middleware, which the host lends and keeps
    driver_selection: DSN scheme resolves a separately imported database/sql driver
    schema_provider: deterministic dialect SQL, check, and apply for api:cli-session-schema
  redis:
    import: _ "popcornwave/sessionstore/redis"
    config_prefix: session.redis
    resources: opens its own client and returns it as Close
    compatibility: requirement:contrib-redis-valkey
  dynamo:
    import: _ "popcornwave/sessionstore/dynamo"
    config_prefix: session.dynamo
    resources: reuses the process client api:dynamo-package installs, so it opens nothing and returns no Close
    requires: api:dynamo-package imported and enabled; the factory refuses otherwise, naming the import
    schema_provider: none, because decision:dynamodb-desired-state-migration has no versioned schema to provide; it registers a table definition instead
    requirement: requirement:dynamodb-session-store
rules:
  - a selected backend that no import registered is a startup error naming the import line to add
  - construct only the selected backend, and open nothing during registration
  - validate only the keys of the selected backend
  - a host asks pw for the selected backend and imports no storage plugin itself
  - reject an RDB DSN whose database/sql driver is not registered
  - middleware source requires enabled api:rdb-middleware
  - middleware source resolves api:request-context-accessors active SQL executor for every store operation
  - middleware source joins an active request transaction and never acquires a second pool connection for that operation
  - compare driver-provided canonical connection identities before opening a dedicated pool
  - never close a lent database; close an owned client through the returned Close
  - schema provider owns only plugin tables and schema-version metadata
deferred:
  - plugin-owned configuration keys; pw still declares session.rdb and session.redis, per data:session-runtime-config
  - a backend whose store is not relational registers a table definition rather than a schema provider, and startup still verifies before serving
```
