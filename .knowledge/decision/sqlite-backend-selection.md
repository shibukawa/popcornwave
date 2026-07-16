---
id: decision:sqlite-backend-selection
type: decision
title: SQLite Backend Selection
---
contrib/database/sqlite preserves one API and driver name while build constraints select the runtime backend.

```yaml
status: accepted
facade: contrib/database/sqlite
driver_name: sqlite
selection:
  - constraint: "tinygo || force_tinygo_logic"
    backend: requirement:contrib-cgosqlite
  - constraint: "!tinygo && !force_tinygo_logic && cgo"
    backend: github.com/mattn/go-sqlite3
  - constraint: "!tinygo && !force_tinygo_logic && !cgo"
    backend: modernc.org/sqlite
registration:
  - every selection exposes DriverName equal to sqlite
  - facade import registers exactly one driver under sqlite
  - mattn native sqlite3 registration may remain but is not the facade contract
compatibility:
  - common API and portable DSN subset remain identical across selections
  - backend-specific DSN parameters are not part of the facade contract
  - decision:force-tinygo-logic selection requires a working C toolchain for SQLite tests
tests:
  - host Go with CGo enabled selects mattn
  - host Go with CGo disabled selects modernc
  - forced host Go selects requirement:contrib-cgosqlite
  - TinyGo selects requirement:contrib-cgosqlite
  - shared create, insert, query, prepared statement, transaction, rollback, null, and cancellation fixtures pass
```
