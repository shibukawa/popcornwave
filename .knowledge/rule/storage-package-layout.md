---
id: rule:storage-package-layout
type: rule
title: Storage Package Layout
---
A storage backend is one import path at the top level, and a SQL backend is one package per engine under it.

```yaml
shape: <family>/<backend>
families:
  database: the connection itself, per rule:rdb-dsn-resolution
  sessionstore: api:session-store records
  authstate: single-use requirement:contrib-auth-state ceremony records
depth:
  rule: two segments, so an application reads the family and the backend and nothing else
  rejected: contrib/ and plugin/ prefixes, which added a segment that named the framework rather than the storage
sql_backends:
  packaging: one package per engine, named for the dialect the DSN scheme resolves to
  shared_core: the family package holds the store, the bounds, and every statement whose only difference is placeholder style
  engine_package: supplies what the engines genuinely disagree about, and registers itself from init
  disagreements:
    - DDL types and key lengths
    - the upsert clause
    - the bounded delete, since not every engine accepts a subquery on its own target or a LIMIT
    - the catalog query behind a schema check
    - whether a single-use read is one RETURNING statement or a locking transaction
selection:
  engine: never configured twice; the store takes the dialect the DSN already resolved to
  linking: decision:import-registered-session-plugins, so a project links the engine it runs and no other
  missing: a startup error naming the import line to add
non_goals:
  - one package that links every engine
  - a dialect abstraction that hides which statements differ
  - translating one engine's DDL into another
```
