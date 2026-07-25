---
id: api:transaction-runner
type: api
title: Transaction API
---
pw.Transaction scopes database work through a child context so generated SQL functions automatically use the active transaction.

```yaml
surface:
  - Transaction(context.Context, func(context.Context) error) error
behavior:
  - read *sql.DB through api:request-context-accessors
  - begin *sql.Tx with the caller context
  - create a child context whose active SQL executor is that transaction
  - call the function with the child context
  - rollback on returned error or panic
  - propagate or convert a panic according to framework recovery policy after rollback
  - commit only after the function returns nil
  - return begin, callback, or commit failure
rules:
  - flow:sql-generation functions receive the child context and select its transaction
  - no repository type is required
  - an initially nested call reuses the active transaction
  - savepoint-backed nesting may be added later
  - recoverable application error mapping occurs outside this runner
  - callers cannot install an arbitrary executor or mutate capsule fields
```
