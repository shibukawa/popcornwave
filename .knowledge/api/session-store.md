---
id: api:session-store
type: api
title: Session Store Contract
---
Session backends persist data:session-record values behind one context-aware contract, and never learn what any slot in them means.

```yaml
placement_of: the session.Private tier of requirement:state-storage-tiers
plugin_surface:
  - RawStore is the non-generic contract a backend implements over already encoded payloads
  - RawRecord carries the encoded payload beside the record timestamps
  - session.Backend bundles a RawStore with its optional Close and Prune, per api:session-backend-plugin
surface:
  - Store.Put(context.Context, keyHash, Record) error
  - Store.Get(context.Context, keyHash) returns Record
  - Store.Touch(context.Context, keyHash, lastSeenAt, idleExpiresAt) error
  - Store.Delete(context.Context, keyHash) error
  - RequestBinder.BindRequest(context.Context, http.ResponseWriter, *http.Request) context.Context, optional
request_binding:
  purpose: a store whose records live in the browser reaches the request and the response through it
  caller: api:session-manager, before every store call it makes on behalf of a request
  backend_stores: implement nothing and receive the context unchanged
codec:
  scope: the api:session-registry slot values only, one codec per slot
  reason: record timestamps stay in backend fields so renewal never rewrites the payload
  default: session.JSONCodec[T] per slot
  seam: the host encodes and decodes; a backend receives one opaque byte payload and never sees an application type
  implemented: session.Typed[T](RawStore, Codec[T]) returns Store[T] over one payload, which the registry replaces
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
backend_selection: requirement:state-storage-tiers
adapters:
  - session.CookieStore, in the session package itself
  - imported RDB plugin over database/sql
  - imported Redis-compatible plugin through requirement:contrib-redis-valkey
  - imported DynamoDB plugin through requirement:dynamodb-store
cookie:
  backend_name: cookie
  status: implemented
  constructor: session.NewCookieStore[T](session.Codec[T], session.CookieStoreOptions)
  storage: a second browser cookie beside the token cookie, named session.DefaultDataCookieName by default
  protection: sealed under policy:cookie-value-protection, bound to the key hash of its own token
  expiry: the sealed stamp is authoritative over the cookie attributes the client controls
  revocation: none; Delete only expires the client copy, per decision:cookie-session-storage
  bounds: the record must fit the browser cookie budget, and an oversized one is refused at the write
  sweep: none, because nothing accumulates on the server
redis:
  backend_name: redis
  servers: Redis or Valkey
  compatibility: requirement:contrib-redis-valkey
  status: implemented
  session_plugin: popcornwave/sessionstore/redis
  constructor: redis.NewStore[T](go-redis UniversalClient, session.Codec[T], Options)
  key_space: configured prefix and the key hash; the store never scans or enumerates
  expiry: server TTL from the record deadline, with the stored deadline still authoritative on read
  renewal: SET XX, so a renewal never recreates a collected key
  commands: GET, SET with expiry, SET XX, DEL
  expiry_sweep: none; the server collects abandoned records
  client_ownership: the caller opens and closes the client
rdb:
  backend_name: rdb
  session_plugin: popcornwave/sessionstore, with one package per engine under it
  status: implemented
  constructor: sessionstore.NewStore(*sql.DB, Options) with the dialect of the resolved DSN
  owned_table: popcornwave_session
  schema: MigrationSQL publishes the migration file; VerifySchema is the startup check
  schema_ownership: rule:framework-owned-tables
  dialects: sqlite, postgres, and mysql, each contributed by its own blank import
  dialect_scope: an engine package supplies the DDL, the upsert, the bounded delete, and the catalog query; every other statement is shared
  expiry_sweep: Prune removes records that expire without being revoked
  driver_registration: separate popcornwave/database engine import
  guaranteed_driver: requirement:contrib-sqlite
  in_memory: sqlite://:memory:
  future_schemes: require implemented and verified drivers before configuration acceptance
  shared_executor: session.rdb.source middleware uses the pool owned by api:rdb-middleware
dynamo:
  backend_name: dynamo
  session_plugin: popcornwave/sessionstore/dynamo
  status: designed, per requirement:dynamodb-session-store
  owned_table: popcornwave_session, created by requirement:dynamodb-migration rather than by a migration file
  schema: no migration file and no version table; the table definition is generated from the plugin's own tagged type
  expiry_sweep: none, per decision:dynamodb-session-expiry; a record is judged expired on read and removed by TTL
  atomic_touch: the renewal is one conditional UpdateItem, so no read-then-write window exists
  read_consistency: decision:dynamodb-session-read-consistency
  shared_client: the process client installed by api:dynamo-package; there is no dedicated form
plugins: decision:import-registered-session-plugins
extension: applications may supply another Store[T] without changing session middleware
selection: a configured backend is resolved by name through api:session-backend-plugin
```
