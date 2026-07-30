---
id: rule:savepoint-dialect-support
type: rule
title: Savepoint Dialect Support
---
Savepoint capability is a static property of the dialect rule:rdb-dsn-resolution reports, not a runtime probe.

```yaml
statements:
  open: SAVEPOINT {name}
  release: RELEASE SAVEPOINT {name}
  rollback: ROLLBACK TO SAVEPOINT {name}
supported:
  - requirement:contrib-sqlite
  - requirement:contrib-postgresql
  - requirement:contrib-mysql
support_tier_note: every listed dialect is first-class under decision:server-sql-support-tier, so this list and the support tier now agree
unknown_driver:
  default: unsupported
  effect: nested api:transaction-runner calls fail; depth 0 transactions still work
caveats:
  mysql: DDL commits implicitly and drops every open savepoint, so schema changes inside a scope are unsupported
  sqlite:
    - a scope holds one connection, so concurrent writers to one file serialize
    - a second write transaction on the same file waits, then fails with database is locked
    - parallel writing tests need one file per test or a server database
verification:
  - open, release, and rollback fixtures per supported driver
  - nested rollback leaves outer work committable
  - unsupported driver returns an explicit nesting error
```
