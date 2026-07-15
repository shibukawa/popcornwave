---
id: requirement:contrib-sqlite
type: requirement
title: TinyGo SQLite Driver
---
contrib/database/sqlite is conditional on a successful TinyGo C integration spike and exposes a restricted database/sql driver over SQLite 3.

```yaml
package: contrib/database/sqlite
gate:
  required_before_commitment:
    - compile pinned SQLite amalgamation with TinyGo on linux amd64 and arm64
    - prove callbacks and pointer ownership under repeated queries
    - prove file I/O, locking, and context interruption
    - measure binary size impact
implementation_if_gate_passes:
  - pinned SQLite amalgamation built from source
  - CGo isolated inside driver package
  - file and in-memory databases
  - prepared statements and transactions
  - sqlite3_interrupt for context cancellation
  - serialized access per connection
security:
  - extension loading omitted
  - shared cache disabled
  - URI parameters allowlisted
  - SQL length and busy timeout configurable and bounded
deferred:
  - pure-Go SQLite reimplementation
  - loadable extensions
  - user-defined Go callbacks
fallback_if_gate_fails: mark unsupported without blocking requirement:contrib-postgresql or requirement:contrib-mysql
api: https://sqlite.org/capi3ref.html
```
