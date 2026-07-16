---
id: requirement:contrib-database
type: requirement
title: TinyGo Database Drivers
---
contrib/database provides portable SQLite and shared database/sql helpers; decision:server-sql-support-tier keeps PostgreSQL and MySQL outside first-class support.

```yaml
packages:
  core: contrib/database
  first_class_drivers:
    - requirement:contrib-sqlite
  non_first_class_research:
    - requirement:contrib-postgresql
    - requirement:contrib-mysql
contract: data:database-driver-contract
strategy:
  - implement database/sql/driver interfaces supported by TinyGo
  - use database/sql pooling and transactions when target smoke tests pass
  - keep wire protocol and C adapter packages independently importable
  - select SQLite through decision:sqlite-backend-selection
  - enforce decision:server-sql-support-tier in documentation, generation, and acceptance
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
