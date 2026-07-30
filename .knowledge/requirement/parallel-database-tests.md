---
id: requirement:parallel-database-tests
type: requirement
title: Parallel Database Tests
---
Tests share one database and stay independent by running each test inside its own transaction that is always rolled back, while framework transactions inside the test become savepoints.

```yaml
goal: run database-backed tests with t.Parallel against one prepared database without per-test database provisioning
mechanism:
  test_boundary: api:test-run opens one *sql.Tx per test and rolls it back at cleanup
  application_boundary: api:transaction-runner and api:rdb-middleware auto transaction nest as savepoints under decision:savepoint-nested-transaction
  visibility: no test writes ever commit, so no test observes another test's rows
flow: flow:test-transaction-isolation
switch:
  option: api:test-run WithTransaction RunOption
  default: off, opt in per test server
  off_guidance: run with test parallelism 1 and clean data explicitly
  rationale: drivers without savepoint support cannot host framework transactions inside a test transaction
engines:
  scope: every decision:server-sql-support-tier first-class engine supports savepoints, so the mechanism works on all three
  sqlite: parallel writers to one file serialize and then fail, per the rule:savepoint-dialect-support caveat, so a writing suite needs one file per test
  server: one shared server database hosts parallel tests directly, which is the case sqlite could not serve
  mysql: DDL inside a test transaction commits implicitly and drops the savepoints, so schema work stays outside the test transaction
test_author_responsibility:
  - avoid unique-key and row-lock collisions between parallel tests by using per-test distinct data
  - size the pool for at least one connection per concurrently running test, and against the server's own connection limit
  - do not assert on committed state, since the test transaction never commits
  - apply schema once before parallel tests, outside any test transaction
  - keep application code resolving its executor from the request context
acceptance:
  - nested api:transaction-runner rollback leaves outer test transaction usable
  - a passing and a failing test both leave the database unchanged
  - parallel tests on one shared database produce stable results
  - transaction option off restores plain pooled *sql.DB behavior
non_goals:
  - per-test database or schema provisioning
  - cross-test isolation beyond the database isolation level
  - distributed transactions
```
