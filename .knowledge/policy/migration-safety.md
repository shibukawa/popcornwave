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
  - any rollback action
startup_apply:
  default: disabled
  enable: explicit configuration plus an explicit non-default acknowledgement
  scope: forward-only apply
rollback:
  published: api:cli-migrate down and down-to are supported operator actions
  rules:
    - an interactive invocation prompts with the target version and the migrations to be reversed
    - a non-interactive invocation requires the explicit confirmation flag
    - the development loop, startup apply, and tests never roll back automatically
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
integrity:
  - an applied migration file must not be edited
  - a recorded version with no matching source is an error, not a warning
  - a missing intermediate version is an error
observability:
  - log each applied version, direction, and duration
  - report the resulting version on both success and failure
```
