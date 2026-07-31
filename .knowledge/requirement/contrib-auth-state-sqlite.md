---
id: requirement:contrib-auth-state-sqlite
type: requirement
title: SQLite Authentication State Store
---
authstate/sqlite implements requirement:contrib-auth-state over the portable SQLite facade for durable single-node deployments.

```yaml
package: authstate/sqlite
database: requirement:contrib-sqlite
minimum_sqlite: 3.35 for DELETE RETURNING
public_api:
  - NewStore[T](*sql.DB, api:auth-state-codec, Options)
  - EnsureSchema(context) creates or validates the owned table
  - Prune(context, before, limit) removes a bounded expired batch
  - Store[T] implements authstate.Store[T]
options:
  - required bounded namespace
  - injectable clock
  - maximum key, encoded payload, and prune batch sizes with hard caps
record: data:auth-state-record
schema:
  table: popcornwave_authstate
  columns:
    - namespace TEXT NOT NULL
    - key TEXT NOT NULL
    - expires_at_ms INTEGER NOT NULL
    - payload BLOB NOT NULL
  primary_key: namespace and key
  rule: fail initialization when an existing table has an incompatible schema
put:
  - encode and bound payload before SQL mutation
  - one conditional UPSERT replaces an expired collision but never an unexpired record
  - zero affected rows maps to ErrAlreadyExists
take:
  - one DELETE RETURNING statement atomically consumes expiry and payload
  - no returned row maps to ErrNotFound
  - validate bounds and expiry before decode
  - malformed, expired, or undecodable rows remain consumed
errors:
  - semantic failures map to requirement:contrib-auth-state errors
  - sanitized driver and locking failures wrap ErrUnavailable
maintenance:
  - Prune deletes only the configured namespace
  - each call is ordered by expiry and bounded by limit
  - application schedules pruning; Put removes an expired same-key collision
security:
  - database file permissions, backups, and volume encryption protect payload at rest
  - SQL text, errors, and metrics never contain keys or payloads
  - busy timeout and context cancellation remain bounded
```
