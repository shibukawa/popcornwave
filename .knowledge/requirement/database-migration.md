---
id: requirement:database-migration
type: requirement
title: Versioned Database Migration
---
Versioned migrations are the single relational schema mechanism of Popcorn Wave, running identically from the CLI, the development loop, and isolated tests.

```yaml
scope:
  covers: every rule:rdb-dsn-resolution engine
  excludes: requirement:dynamodb-store, whose schema is applied by requirement:dynamodb-migration
  why_split: system:goose is a SQL engine and host-only, and a DynamoDB table shape is readable back through DescribeTable, so neither the engine nor the version table transfers
  why_the_scopes_differ:
    here: a relational schema normally sits outside what infrastructure tooling manages, so these migrations own production
    there: a DynamoDB table reads as part of the infrastructure and is created by the same tooling as the queue and the bucket, so that mechanism creates only in development and verifies in production
    shared_shape: both are still one desired schema, one command, and one development-loop step
  shared: the api:cli-migrate verbs, the api:cli-dev apply step, and the api:test-run integration, so an operator meets one mechanism
engine: system:goose
sources: data:migration-source
surfaces:
  cli: api:cli-migrate
  application: api:migration-runner
  development: api:cli-dev automatic apply
  test: api:test-run migration option
execution_model: decision:migration-execution-split
engine_linkage: decision:goose-migration-engine
flow: flow:database-migration
safety: policy:migration-safety
configuration:
  effective_database: decision:config-driven-database
  tooling_paths: data:project-config
removes:
  command: pw schema-init
  convention: dbschema directory and its unversioned lexical SQL
  test_option: api:test-run WithMigrations and WithMigrationsFS
  scaffold: the initial schema example moves to migration version 1
  rule: no compatibility path is kept; an existing project rewrites its schema as migrations
acceptance:
  - one migration source tree drives CLI, dev, and test apply
  - the same effective DSN resolution serves runtime and migration
  - a host Go application or test applies migrations in process
  - a TinyGo application or test applies migrations through the pw child process path
  - pw dev applies pending migrations before the application accepts requests
  - api:test-run installs the migrated schema into its isolated copied database, including sqlite://:memory:
  - api:test-run installs the same schema into a configured server database by applying it there, and a second run reuses it
  - repeated apply on a current database performs no schema change
  - down and down-to reverse the newest applied migrations under confirmation
  - a failed migration returns nonzero and leaves recorded version state consistent
  - credentials never appear in output, logs, process arguments, or errors
non_goals:
  - ORM or schema diffing
  - automatic migration authoring from Go types or .pw.sql sources
  - multi-node coordinated online migration
  - data backfill orchestration
```
