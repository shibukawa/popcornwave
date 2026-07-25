---
id: flow:database-migration
type: flow
title: Database Migration Flow
---
Every trigger resolves the same effective configuration and migration source, then applies pending versions through one runner before the application serves requests.

```yaml
triggers:
  - api:cli-migrate explicit invocation
  - api:cli-dev automatic apply
  - api:test-run isolated apply
  - api:migration-runner startup apply when explicitly enabled
steps:
  - resolve the project root and data:project-config migration settings
  - resolve the effective database through decision:config-driven-database
  - load data:migration-source from disk or an embedded fs.FS
  - validate ordering, duplicate versions, and annotation syntax before connecting
  - open a connection and read the applied version state
  - compute the pending set for the requested action
  - apply each pending migration in version order
  - record each applied version before starting the next
  - report previous version, current version, and applied list
  - close resources owned by this run
dev_loop:
  position: after api:cli-generate and before the application process starts
  rerun: the migration directory is watched, and a change reapplies pending migrations before restart
  failure: report diagnostics, skip the restart, and keep the developer loop alive per api:cli-dev
test_run:
  scope: the copied and isolated database of api:test-run
  lifetime: applied once per TestRun before the server starts
  install: replay a cached data:migration-snapshot per decision:test-migration-snapshot
failure:
  - a validation failure stops before any connection is opened
  - a statement failure rolls back the current migration where the dialect supports transactional DDL
  - a partially applied non-transactional migration is reported with its version so the operator can repair it
  - every failure path returns nonzero and leaves earlier applied versions recorded
idempotence: a run with no pending migrations performs no schema change and reports the current version
```
