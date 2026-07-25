---
id: flow:test-transaction-isolation
type: flow
title: Test Transaction Isolation
---
Per-test transaction lifecycle that makes requirement:parallel-database-tests work on one shared database.

```yaml
setup:
  - shared database is prepared once, schema applied outside any test transaction
  - api:test-run copies configuration and opens or reuses the pool
  - transaction RunOption enabled: BEGIN one *sql.Tx and install data:transaction-scope at depth 0
  - the scope is injected into every request context created by that test server
request:
  - handler SQL resolves the scope transaction as its executor
  - each api:transaction-runner call pushes a savepoint per decision:explicit-transaction-boundary
  - a released savepoint keeps the work visible to the test but never commits it
teardown:
  - Server.Close stops the server, rolls back the depth 0 transaction, then closes the pool
  - a registered cleanup runs it after the test finishes, pass or fail
  - rollback failure is reported as a test failure
parallel:
  - each test owns one scope, one connection, and one rollback
  - tests may call t.Parallel because no test commits
  - statements inside one scope serialize on its connection
constraints:
  - pool max_open_conns must allow one connection per concurrently running test
  - DDL inside a test transaction is unsupported per rule:savepoint-dialect-support
  - code opening its own pool or connection bypasses the scope and leaks committed data
```
