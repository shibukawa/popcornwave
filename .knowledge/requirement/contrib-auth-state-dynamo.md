---
id: requirement:contrib-auth-state-dynamo
type: requirement
title: DynamoDB Authentication State Store
---
authstate/dynamo implements requirement:contrib-auth-state over requirement:dynamodb-store, so a deployment with no relational database can run the passkey, OAuth, and OIDC ceremonies.

```yaml
package: authstate/dynamo, beside the memory, sqlite, postgres, mysql, and redis adapters
store: requirement:dynamodb-store
selected_by: decision:auth-backend-selection, as one of the four stores requirement:dynamodb-auth-backend moves together
implemented:
  built: 2026-08-05, together with the rest of requirement:dynamodb-auth-backend
  shape: a raw store over encoded payloads, because plugin/auth keeps three kinds of ceremony record and two of their types are unexported; NewStore puts a codec back on for a caller that owns one
  raw_contract: authstate.RawStore and authstate.Typed, added for this
  was_blocked_by: the adapter itself, the absent configuration seam, the middleware.rdb gate, and the allowlist having no store seam; all four are now answered
record: data:auth-state-record
public_api:
  - NewRawStore(Options) returns the authstate.RawStore a backend supplies
  - NewStore[T](api:auth-state-codec, Options) wraps it for a caller that owns a codec
client:
  source: the request context, installed by the api:dynamo-package middleware
  no_constructor_argument: unlike the sqlite adapter, which takes a pool, because system:tinybind carries the client in the context from v0.2.10
  missing: the driver's no-client error, surfaced as requirement:contrib-auth-state ErrUnavailable
table:
  declared_name: popcornweb_authstate, per rule:framework-owned-tables
  key: one partition key holding the namespace and the correlation key joined; no sort key
  why_joined:
    rejected: namespace as the partition key and the correlation key as the sort key, which would make one namespace one partition
    reason: every ceremony of one protocol shares a namespace, so a login spike would concentrate on a single partition
    lost_by_joining: a cheap namespace-scoped listing, which nothing needs once no prune exists
  definition: registered through decision:dynamodb-table-registry, so requirement:dynamodb-migration creates it in development and verifies it everywhere
put:
  operation: PutItem conditioned on no unexpired record existing for that key
  condition: the item is absent, or its stored expiry has already passed
  effect: an expired collision is replaced and an unexpired one is not, which is the sqlite adapter's rule reached in one request instead of a conditional upsert
  rejection: the driver's conditional-check error maps to ErrAlreadyExists
  bounds: the encoded payload is bounded before the request, and a record over the DynamoDB item limit is refused with the limit named
take:
  operation: DeleteItem asking for the old item, which returns what it removed
  atomicity: one request removes and returns, so the contract's single-use guarantee needs no read followed by a delete
  parallel: this is the same shape as the sqlite adapter's DELETE RETURNING, and the reason both adapters can promise it
  no_row: ErrNotFound
  after_removal: expiry is validated, then bounds, then decode; a malformed, expired, or undecodable record stays consumed, per data:auth-state-record
expiry:
  authority: the stored deadline, checked on Take; a record past it is never returned
  removal: DynamoDB TTL on a second-precision attribute maintained beside the millisecond deadline the contract carries
  configuring_it: deployment tooling, per decision:dynamodb-operational-configuration
  no_prune: decision:dynamodb-session-expiry applies unchanged; a scan to find expired ceremony records would cost more than the records
  contrast: the sqlite adapter publishes Prune because a bounded DELETE is cheap there, and this adapter publishes none
security:
  - namespace, key, expiry, and payload never enter a log or a stable error, per data:auth-state-record
  - this table is a rule:framework-owned-tables table, so policy:query-log-safety records nothing for it and no payload can reach a diagnostic artifact
  - payload encryption stays an explicit codec or deployment responsibility
acceptance:
  - a passkey, OAuth, and OIDC ceremony completes with this adapter selected and no relational database configured
  - a second Put for an unexpired key returns ErrAlreadyExists without overwriting
  - a Put over an expired key of the same name succeeds
  - two concurrent Takes of one key return the value exactly once
  - a Take after expiry returns the contract error and leaves nothing behind
  - the table is created by the same development migration run as every other table
non_goals:
  - a prune or sweep operation
  - namespace-scoped enumeration
  - sharing one table with requirement:dynamodb-session-store, which would be the single-table design system:tinybind declines
```
