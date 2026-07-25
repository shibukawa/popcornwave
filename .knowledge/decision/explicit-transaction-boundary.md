---
id: decision:explicit-transaction-boundary
type: decision
title: Explicit Transaction Boundary
---
Transaction boundaries come only from api:transaction-runner; api:rdb-middleware never begins a request transaction and the middleware.rdb.auto_transaction setting is removed.

```yaml
status: accepted
removed:
  key: middleware.rdb.auto_transaction
  effect: no config key, flag, scaffold field, or RDBConfig field
rationale:
  - decision:savepoint-nested-transaction makes an explicit call safe to nest, so an implicit outer transaction adds no convenience
  - a request-wide transaction hides its boundary from the code that depends on it
  - implicit commit on status below 400 couples HTTP status to data durability
  - buffering every response to defer the commit conflicts with streaming, hijacking, and early flush
  - handlers that need no transaction should not hold a connection for the whole request
consequences:
  - application code wraps its own mutations in api:transaction-runner
  - reads outside a transaction use the pool directly
  - api:test-run keeps its own depth 0 transaction for requirement:parallel-database-tests, which is a test-only owner
  - concept:classic-web-style mutation guidance names the explicit call
```
