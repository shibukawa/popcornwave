---
id: api:transaction-runner
type: api
title: Transaction API
---
pw.Transaction scopes database work through a child context so generated SQL functions automatically use the active transaction, and nests through savepoints.

```yaml
surface:
  - Transaction(context.Context, func(context.Context) error) error
state: data:transaction-scope
group:
  source: the effective group of the caller context, per api:database-selection
  explicit: SelectDB on the context handed here, which is the call a single statement already uses
  form: the runner takes no group option, so a group has one spelling and no option-beats-context precedence to document
  readonly: a group whose selected connection is readonly begins a read-only transaction, and sqlbind rejects a write inside it
option_placement:
  here: only what exists because a transaction exists, such as an isolation level or a retry policy
  api:database-selection: anything that also governs a statement outside a transaction, which is why the group lives there
  form_when_added: a variadic option after the callback, so a two-argument call keeps compiling
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
  - fail when the caller context selects a group other than the active scope group, per decision:grouped-database-connections
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
