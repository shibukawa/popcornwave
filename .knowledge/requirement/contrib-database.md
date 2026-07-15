---
id: requirement:contrib-database
type: requirement
title: TinyGo Database Drivers
---
contrib/database provides configuration helpers and database/sql drivers for PostgreSQL, MySQL, and feasibility-gated SQLite without adding an ORM.

```yaml
packages:
  core: contrib/database
  drivers:
    - requirement:contrib-postgresql
    - requirement:contrib-mysql
    - requirement:contrib-sqlite
contract: data:database-driver-contract
strategy:
  - implement database/sql/driver interfaces supported by TinyGo
  - use database/sql pooling and transactions when target smoke tests pass
  - keep wire protocol and C adapter packages independently importable
core_helpers:
  - parse explicit driver configuration without reflection
  - open and ping with context deadline
  - apply bounded pool settings
  - redact credentials from errors and logs
non_goals:
  - ORM
  - migrations engine
  - query builder
  - distributed transactions
evidence: https://tinygo.org/docs/reference/lang-support/stdlib/#databasesql
```
