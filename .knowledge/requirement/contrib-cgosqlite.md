---
id: requirement:contrib-cgosqlite
type: requirement
title: Native TinyGo SQLite Driver
---
contrib/database/cgosqlite provides a native database/sql driver over a pinned SQLite amalgamation for TinyGo and forced compatibility builds.

```yaml
package: contrib/database/cgosqlite
contract: data:database-driver-contract
driver_name: sqlite
source:
  - pin official sqlite3.c and sqlite3.h with version and SHA-256
  - statically link the amalgamation without a system libsqlite3 dependency
  - keep OS-specific C flags in build-tagged Go files without conditional #cgo directives
required_c_api:
  - sqlite3_open_v2 and sqlite3_close_v2
  - sqlite3_prepare_v3, sqlite3_step, sqlite3_reset, and sqlite3_finalize
  - sqlite3_bind_null, sqlite3_bind_int64, sqlite3_bind_double, sqlite3_bind_text, and sqlite3_bind_blob
  - sqlite3_column_type, sqlite3_column_int64, sqlite3_column_double, sqlite3_column_text, sqlite3_column_blob, and sqlite3_column_bytes
  - sqlite3_changes64 and sqlite3_last_insert_rowid
  - sqlite3_errcode, sqlite3_extended_errcode, and sqlite3_errmsg
  - sqlite3_busy_timeout and sqlite3_interrupt
behavior:
  - support file and in-memory databases
  - serialize operations per connection
  - support prepared statements and transactions
  - map nil, int64, float64, bool, string, []byte, and time.Time
  - copy SQLite-owned text and blob memory before step, reset, or finalize invalidates it
  - finalize every statement and close every connection idempotently
  - interrupt active work when context cancellation wins
  - return driver.ErrBadConn only when a connection cannot be safely reused
portable_dsn:
  - plain file path
  - :memory:
  - allowlisted file URI with mode, cache, immutable, and busy_timeout parameters
security:
  - omit loadable extensions
  - disable double-quoted string literals
  - disable shared cache by default
  - bound SQL length, busy timeout, and retained values
  - omit Go callbacks and user-defined functions in the first release
native_only:
  - no ebitengine/purego
  - no embedded WASM runtime
  - no dynamic system SQLite loading
verification:
  - host CGo and TinyGo results match shared fixtures
  - Linux amd64 and arm64 compile and execute
  - repeated prepare, bind, step, reset, finalize, cancellation, locking, and reopen tests pass
  - binary size and query latency are recorded against selected host backends
source_api: https://sqlite.org/c3ref/intro.html
```
