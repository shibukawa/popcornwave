---
id: requirement:dynamodb-session-store
type: requirement
title: DynamoDB Session Backend
---
sessionstore/dynamo implements api:session-store over requirement:dynamodb-store, so a deployment with no relational database can still hold login sessions.

```yaml
motivation:
  - requirement:dynamodb-store allows a project with no rdb section, and today such a project cannot log anyone in
  - a serverless deployment already paying for DynamoDB should not add a relational database for one table
  - api:session-store is a four-operation key-value contract, which is what this store is
plugin:
  import: popcornwave/sessionstore/dynamo
  backend_name: dynamo
  config_prefix: session.dynamo
  registration: api:session-backend-plugin; the import registers the factory under the backend name, so session.backend selects it
  requires: api:dynamo-package imported and enabled, the way the rdb backend requires api:rdb-middleware
  client: the process client that middleware installs; the backend borrows it, so it returns no Close and opens nothing
  raw_store: it implements session.RawStore over already encoded payloads, so it never sees the application payload type; the host adds it back with session.Typed
table:
  declared_name: popcornwave_session, per rule:framework-owned-tables
  deployed_name: resolved by rule:dynamodb-table-naming like any other table, so a test prefix reaches it unchanged
  key: the record key hash as the partition key; no sort key
  definition_source: registered through decision:dynamodb-table-registry when the package is imported, so requirement:dynamodb-migration creates it with every other table
  handwritten: the definition and the item mapping are written rather than generated, decided on implementation 2026-08-01
  why: this package is the only reader and writer of the item, so there is no drift for a generated codec to close, and generating for one internal type would put a generation step in the framework's own build
  what_replaces_the_guarantee: one constant names the key attribute, and both the definition and the item read it
  no_migration_file: this store has none, per decision:dynamodb-desired-state-migration; the rule:framework-owned-tables migration-file convention is the SQL half of the same idea
operation_mapping:
  Put: PutItem, which replaces one item atomically and needs no condition
  Get: GetItem, with decision:dynamodb-session-read-consistency deciding the consistency
  Touch: UpdateItem conditioned on the item existing and not being dead, which is stronger than the SQL path's read-then-write
  Touch_clamping: the caller has already clamped the renewal to the absolute expiry, so dead_at takes the new idle expiry outright; a DynamoDB update expression has no conditional operator to clamp with anyway
  Delete: DeleteItem, idempotent by construction
  errors: a miss is the driver's item-not-found sentinel, reported as the contract's not-found; nothing converts it to a zero record
stored_attributes:
  source: data:session-record
  data: the codec payload as a binary attribute, so no timestamp rewrite touches it
  timestamps: epoch-second numeric attributes, since DynamoDB has no time type
  dead_at: the minimum of the absolute and idle expiry, maintained by Put and Touch, and the attribute a deployment points TTL at
  reason_for_dead_at: TTL reads one numeric attribute, and the record dies at whichever expiry comes first
expiry: decision:dynamodb-session-expiry
bounds:
  item_size: the plugin rejects a record over the DynamoDB item limit before sending, naming the limit, rather than letting the service answer with a validation error
  write_rate: every Touch is a write, so the data:session-runtime-config renewal_interval bounds cost more visibly here than on a relational store
ttl_is_not_ours:
  what: nothing in the framework enables or verifies expiry, per decision:dynamodb-operational-configuration
  effect: the store is correct without TTL and unbounded in size without it, and the guide says so
  not_blocked: this needs no driver change; the store is complete as designed
acceptance:
  - a project with middleware.rdb absent and middleware.dynamo enabled logs a user in, keeps the session across requests, and logs out
  - the session table is created by the same pw migrate run that creates application tables
  - startup refuses to serve when the table is absent, naming the table and the command that creates it
  - Touch on an expired record fails without reviving it, in one request
  - Delete twice succeeds twice
  - a record over the item limit is refused with the limit named, before any request
  - two parallel test servers with different prefixes never observe each other's sessions
  - no request this store issues produces a query-diagnostics record, per policy:query-log-safety, so no cookie value, key hash, stored payload, or CSRF secret can reach one
non_goals:
  - a sweep of expired records, per decision:dynamodb-session-expiry
  - enabling or verifying TTL on the deployed table
  - a dedicated client or a second region for sessions; the process client is the only one
  - sharing one table with requirement:contrib-auth-state records, which would be the single-table design system:tinybind declines
  - global tables or multi-region replication, which the driver does not expose
```
