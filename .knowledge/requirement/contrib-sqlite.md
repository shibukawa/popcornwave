---
id: requirement:contrib-sqlite
type: requirement
title: Portable SQLite Facade
---
system:tinygodriver exposes one database/sql SQLite contract while decision:sqlite-backend-selection chooses the build-specific implementation.

```yaml
package: github.com/shibukawa/tinygodriver/database/sql/sqlite
driver_name: sqlite
selection: decision:sqlite-backend-selection
tinygo_backend: requirement:contrib-cgosqlite
public:
  - DriverName constant
  - import registers exactly one sqlite driver
supported:
  - file and in-memory databases
  - prepared statements and transactions
  - context-aware exec, query, prepare, and begin
  - null and data:database-driver-contract scalar values
portable_dsn:
  - plain file path
  - :memory:
  - allowlisted file URI subset
rules:
  - application code imports only this facade when backend portability matters
  - backend-specific connection types, pragmas, and DSN parameters are not portable API
  - shared behavior tests run unchanged against every selected backend
non_goals:
  - ORM
  - migrations
  - cross-backend byte-identical error messages
api: https://sqlite.org/capi3ref.html
```
