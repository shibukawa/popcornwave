---
id: policy:migration-safety
type: policy
title: Migration Safety
---
Automatic migration is limited to environments the developer controls, and every rollback is an explicit, acknowledged action.

```yaml
automatic_allowed:
  - api:cli-dev development loop
  - api:test-run isolated database
automatic_forbidden:
  - production application startup by default
  - any rollback action outside the api:cli-dev loop
startup_apply:
  default: disabled
  enable: explicit configuration plus an explicit non-default acknowledgement
  scope: forward-only apply
rollback:
  published: api:cli-migrate down and down-to are supported operator actions
  rules:
    - an interactive invocation prompts with the target version and the migrations to be reversed
    - a non-interactive invocation requires the explicit confirmation flag
    - startup apply and tests never roll back automatically
    - the api:cli-dev loop does, and only to reach an edited migration; it is bounded to a development database the developer is editing the schema of, it announces the versions it reverses, and it stops on a migration with no usable Down like every other caller
    - a migration without a usable Down section fails before any statement runs
credentials:
  - never pass a DSN as a child process argument
  - use a single-use environment variable or stdin handshake for decision:migration-execution-split delegation
  - redact DSN credentials from output, logs, and errors as required by data:middleware-runtime-config
concurrency:
  - one migrator at a time per database
  - goose advisory locking is dialect-specific and absent for the first-class SQLite tier
  - concurrent application instances must not rely on startup apply for coordination
transactions:
  - apply each migration transactionally where the dialect supports transactional DDL
  - a migration marked no-transaction is reported as manually repairable on failure
content_boundary:
  rule: a migration carries schema, not sample rows
  rows_belong_to: requirement:test-data-seeding, through api:cli-seed for a database and api:test-seed for a test
  why_not_a_migration:
    every_environment: one sequence is applied everywhere, so development fixtures reach production the first time production catches up
    immutable: an applied file cannot be edited, so a wrong or stale row is corrected only by a later version that deletes it
    forward_only: startup apply never rolls back, so there is no supported way to take those rows out again
    no_expected_form: a data:seed-dataset can be reapplied and compared against; a migration runs once and answers no question about state
  permitted_rows: rows a schema change needs to be correct, such as backfilling a new column or the lookup values a new foreign key points at, which belong to the version that created the need
  test: whether the application would be wrong without the rows, or only less interesting to look at
  documented_in: the migrations guide and the database tutorial chapter, because this is decided while writing the first migration and discovered much later
integrity:
  - an applied migration file must not be edited, except while api:cli-dev is watching it, which is the phase before the file has been applied anywhere else
  - a recorded version with no matching source is an error, not a warning
  - a missing intermediate version is an error
observability:
  - log each applied version, direction, and duration
  - report the resulting version on both success and failure
```
