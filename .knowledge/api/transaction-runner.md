---
id: api:transaction-runner
type: api
title: Transaction API
---
pw.Transaction scopes database work through a child context so generated SQL functions automatically use the active transaction, and nests through savepoints.

```yaml
surface:
  - Transaction(context.Context, func(context.Context) error, ...TxOption) error
  - OnGroup(group string) TxOption
state: data:transaction-scope
group:
  default: the effective group of the caller context, per api:database-selection
  explicit: OnGroup names a data:database-connection-set group for this call and its children
  form: a variadic option, so an existing two-argument call keeps compiling and keeps its meaning
  readonly: a group whose selected connection is readonly begins a read-only transaction, and sqlbind rejects a write inside it
depth_0:
  - resolve the group, then its memoized connection
  - begin *sql.Tx on that connection with the caller context
  - create a child context carrying a new scope whose active SQL executor is that transaction
  - call the function with the child context
  - rollback on returned error or panic
  - commit only after the function returns nil
  - return begin, callback, or commit failure
nested:
  model: decision:savepoint-nested-transaction
  - detect the active scope instead of beginning a transaction
  - fail when OnGroup names a group other than the active scope group, per decision:grouped-database-connections
  - fail when rule:savepoint-dialect-support marks the driver unsupported
  - fail when the scope is already marked failed
  - open SAVEPOINT pw_sp_{depth} and call the function with a child context at that depth
  - release the savepoint after the function returns nil
  - rollback to the savepoint, release it, and return the error on failure or panic
  - mark the scope failed when release or rollback fails
panic:
  - unwind the current depth first, then propagate or convert the panic per framework recovery policy
rules:
  - flow:sql-generation functions receive the child context and select its executor
  - no repository type is required
  - inner rollback keeps the outer transaction committable
  - a failed scope makes any later commit fail
  - recoverable application error mapping occurs outside this runner
  - callers cannot install an arbitrary executor, name a savepoint, choose a connection within a group, or mutate capsule fields
  - a scope created by api:rdb-middleware or api:test-run makes the application's first call nested
  - scope adoption unwraps api:instrumented-sql-executor before asserting a concrete transaction type
  - an unknown group name fails here rather than at api:database-selection
```
