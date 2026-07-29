---
id: api:database-selection
type: api
title: Database Group Selection
---
pw.SelectDB pins one data:database-connection-set group onto a context so generated SQL and api:transaction-runner use that group instead of the default.

```yaml
surface:
  - SelectDB(context.Context, group string) context.Context
  - DB(context.Context) (*sql.DB, bool) returns the pool of the effective group
  - OnGroup(group string) TxOption for api:transaction-runner
  - SelectWriteDB(context.Context) pins the resolved write group
  - SelectSessionDB(context.Context) pins the resolved session group
  - RDBConfig.MigrationDSN() reports the migration group DSN for tooling
resolved_pointers:
  owner: policy:connection-group-selection
  reason: a caller that must write should not have to know the deployment topology
effective_group:
  order:
    - the group pinned by the nearest SelectDB
    - the group of the active data:transaction-scope
    - default_group
  property: unpinned SQL inside a transaction stays on the transaction group, so a writer transaction never leaks a read to a replica
executor_resolution:
  - use the active data:transaction-scope transaction when its group equals the effective group
  - otherwise use an explicit sqlbind executor installed in context
  - otherwise use the memoized round-robin connection of the effective group
  - mark the executor read-only when the selected connection is readonly
  - decorate the result per api:instrumented-sql-executor when query diagnostics are enabled
unknown_group:
  form: SelectDB returns a context rather than an error, so the failure is deferred
  effect: the first executor resolution, DB call, or Transaction on that context returns a named unknown-group error
  reason: a group name is data, and a poisoned context fails at the statement that depended on it instead of panicking at selection
  collapsed_set: answers every name, so a single-database deployment never reaches this error
escaping_a_transaction:
  case: SelectDB names a group other than the active scope group
  allowed: only when the named group is readonly
  effect: statements run on that group's pool, outside the transaction and without its atomicity
  writable_target: rejected, because a write outside the open transaction reads as atomic and is not
rules:
  - selection never opens, commits, or rolls back a transaction, per decision:explicit-transaction-boundary
  - a pinned group survives into every child context, including one created by api:transaction-runner
  - the round-robin cursor advances once per group per context chain, so repeated statements reuse one connection
  - callers cannot enumerate the set, read a cursor, or install a connection of their own
  - api:request-context-accessors exposes no group list, and the group name never reaches a log as a secret
```
