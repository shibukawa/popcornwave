---
id: policy:connection-group-selection
type: policy
title: Connection Group Selection
---
Which data:database-connection-set group serves a given piece of work, for application code and for framework-owned features alike.

```yaml
application_read:
  default: default_group
  explicit: api:database-selection
application_write:
  rule: pin the group with api:database-selection, then issue the statement or hand that context to api:transaction-runner
  unpinned: runs on default_group and fails when that group is readonly
framework_write:
  resolution_order:
    - the per-purpose setting for that feature
    - middleware.rdb.write_group
    - the only group holding a writable connection
  ambiguous: startup error naming the unset setting, never a silent pick
  purposes:
    session_store: session.rdb.group, used when session.rdb.source is middleware
    migration: middleware.rdb.migration_group, consumed by api:migration-runner and api:cli-migrate
    seeding: the migration group, so api:test-seed and pw seed write where the schema lives
  accessors: api:database-selection exposes one pinning function per purpose rather than a group registry
tests:
  collapse: api:test-run opens only the resolved migration group and maps every configured group name onto that one pool
  effect: code calling SelectDB with a replica group runs unchanged under test
  reason: a test needs schema and data in one place, and a second pool would break the single test transaction
  shared_rule: the same collapse applies to any single-connection deployment, per data:database-connection-set
rules:
  - a readonly connection never serves a framework-owned write, and configuring one is a startup error
  - the startup summary names the resolved default, write, session, and migration groups
  - a group name is never taken from request input
  - policy:query-log-safety still governs what a data:query-record may carry, and the group label is safe to log
  - policy:migration-safety confirmation rules are unchanged by group selection
```
