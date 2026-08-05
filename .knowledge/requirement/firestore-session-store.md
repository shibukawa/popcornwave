---
id: requirement:firestore-session-store
type: requirement
title: Firestore Session Backend
---
sessionstore/firestore implements api:session-store over requirement:firestore-store, so a Google Cloud deployment with no relational database can hold login sessions.

```yaml
motivation:
  - requirement:firestore-store allows a project with no rdb section, and such a project cannot log anyone in without this
  - api:session-store is a four-operation key-value contract, which is what this store is
  - the DynamoDB backend already proved the shape; what differs here is entirely the write model, per decision:firestore-conditional-writes
plugin:
  import: popcornwave/sessionstore/firestore
  backend_name: firestore
  config_prefix: session.firestore
  registration: api:session-backend-plugin; the import registers the factory under the backend name, so session.backend selects it
  requires: api:firestore-package imported and enabled, the way the dynamo backend requires api:dynamo-package
  client: the process client that middleware installs, reached through EnsureClient at setup; the backend borrows it, so it returns no Close and opens nothing
  raw_store: it implements session.RawStore over already encoded payloads, so it never sees the application payload type; the host adds it back with session.Typed
  config_keys: none of its own, since decision:dynamodb-session-read-consistency has no counterpart and there is no consistency to choose
kind:
  name: popcornwave_session, the same declared name rule:framework-owned-tables gives the relational table and the DynamoDB table
  literal: no resolution step, per decision:firestore-namespace-isolation
  key: datastore.NameKey with the record key hash as the name; no ancestor, so every session is its own entity group and two sessions never contend
  no_creation_step: the kind exists once the first session is written, per decision:firestore-no-schema-application
  handwritten: the entity mapping is written rather than generated, for the reason requirement:dynamodb-session-store gives; this package is the only reader and writer, and generating for one internal type would put a generation step in the framework's own build
  one_constant_per_property: what replaces the guarantee a generated codec would have given
stored_properties:
  source: data:session-record
  data: the codec payload as an Unindexed Blob, so no timestamp rewrite touches it and no index is built over it
  timestamps: datastore.Time properties, since Datastore has a real timestamp type; the epoch-second numeric attributes requirement:dynamodb-session-store carries have no reason to exist here
  dead_at: the minimum of the absolute and idle expiry, maintained by Put and Touch, and the one property a deployment points a TTL policy at
  method_and_version: ordinary indexed properties, small and bounded
  entity_version_field: a separate int64 carrying the Datastore version, which makes the Touch precondition automatic through firestorebind Versioner; it is not data:session-record version, and the two are never read for each other
  unindexed_by_default: every blob and every digest, because system:tinygodriver-firestore silently stops indexing a string over 1500 bytes rather than refusing it, and a property that is sometimes indexed is worse than one that never is
  microsecond_truncation: a stored timestamp truncates to microseconds, which is below the resolution any session deadline is compared at, and the store does not pretend otherwise
operation_mapping:
  Put: one Put, which replaces the entity atomically and needs no precondition
  Get: one Get, strongly consistent by default; a miss is ErrNoSuchEntity, reported as the contract's not-found, and nothing converts it to a zero record
  Touch:
    shape: Load, check aliveness in Go, then Store, whose base-version precondition the loaded version supplies
    why_not_one_request: there is no partial update and no predicate over a stored value, per decision:firestore-conditional-writes
    cost: two round trips per renewal, and the whole record including the payload is rewritten
    what_is_preserved: a renewal never revives an expired or missing record, because the aliveness check runs on the read and the precondition refuses a write against an entity that moved
    clamping: the caller has already clamped the renewal to the absolute expiry, so dead_at takes the new idle expiry outright
    contention: a precondition failure means the session was rotated or deleted concurrently, which is reported as the contract's not-found rather than retried
  Delete: one Delete, idempotent by construction
expiry: decision:firestore-expiry-policy
bounds:
  entity_size: the plugin rejects a record over datastore.MaxEntityBytes before sending, naming the limit, rather than letting the service answer with a validation error
  key_size: a key hash is fixed width and far under datastore.MaxKeyBytes, so this is checked once in a test rather than per write
  write_rate: every Touch is a read and a write, so the data:session-runtime-config renewal_interval bounds cost more visibly here than on a relational store, and more than on DynamoDB, where a renewal is one request
  per_entity_write_ceiling: Firestore sustains roughly one write per second to a single entity, which one session's renewal interval never approaches; it is recorded because a future feature writing to a shared entity would
  key_distribution: a key hash is random, so writes spread across the key space rather than concentrating, which is what the service's ramp-up guidance asks for
cost_comparison_with_dynamodb:
  read: one strongly consistent read, against an eventually consistent read plus a retry on a miss
  renewal: two requests, against one
  reading: this store trades a cheaper read path for a more expensive renewal, and the guide states it where session.renewal_interval is described
acceptance:
  - a project with middleware.rdb absent and middleware.firestore enabled logs a user in, keeps the session across requests, and logs out
  - Touch on an expired record fails without reviving it
  - Touch losing a race with a rotation or a delete reports not-found rather than recreating the record
  - Delete twice succeeds twice
  - a record over the entity limit is refused with the limit named, before any request
  - the stored payload is never indexed, asserted by reading back the property's index exclusion
  - two parallel test servers with different namespaces never observe each other's sessions, per requirement:firestore-test-isolation
  - no request this store issues produces a query-diagnostics record, per policy:query-log-safety, so no cookie value, key hash, stored payload, or CSRF secret can reach one
implemented:
  built: 2026-08-05, in sessionstore/firestore
  entity_version: a separate int64 on the record type, filled by the decoder and read by firestorebind Versioner, so the renewal precondition is automatic rather than assembled at the call site
non_goals:
  - a sweep of expired records, per decision:firestore-expiry-policy
  - applying or verifying a TTL policy on the deployed kind
  - a dedicated client or a second database for sessions; the process client is the only one
  - sharing one kind with requirement:contrib-auth-state-firestore records
  - an ancestor relation between sessions and anything else, which would put unrelated records in one entity group
```
