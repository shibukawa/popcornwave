---
id: requirement:dynamodb-test-isolation
type: requirement
title: Isolated DynamoDB Tests
---
DynamoDB tests stay independent by giving each test server its own table prefix and its own created tables, because the store has no transaction to roll back.

```yaml
problem: requirement:parallel-database-tests isolates SQL tests inside one rolled-back transaction, and system:tinygodriver-dynamodb has no transaction at all
mechanism:
  prefix: api:test-run assigns a unique data:dynamodb-runtime-config table_prefix per test server, per rule:dynamodb-table-naming
  create: the run applies flow:dynamodb-migration into that prefixed set before the first test
  cleanup: the run deletes its own prefixed tables at teardown, which is the one place policy:dynamodb-migration-safety allows a delete, because the run created them and owns the prefix
  parallelism: two test servers never address one table, so t.Parallel needs no further coordination
endpoint:
  development: amazon/dynamodb-local, the same server api:cli-dev starts, per system:tinygodriver-dynamodb
  in_memory: the local server is started with -inMemory and -sharedDb, so a run leaves nothing behind and one credential can see another's tables
  real_account: supported and slow; table creation is asynchronous, so a suite against a real endpoint pays the active-state poll per table
why_the_prefix_reaches_every_call:
  fact: the resolver runs inside the runtime entry, so a declared name in a .pw.dynamo statement and a literal in an item call are both mapped
  effect: a test needs no discipline at the call site; installing the client with a per-run prefix is the whole mechanism
  second_client: a test installs a second context rather than a second signature, which is what makes two servers in one process independent
test_author_responsibility:
  - avoid asserting on a global table list, which sees every parallel run's tables
  - keep the item key distinct per test where a suite deliberately shares one prefix
acceptance:
  - two test servers running in parallel create disjoint table sets and produce stable results
  - a failing test leaves no table behind
  - a call naming a table literally still reaches the run's own prefixed table, because the resolver maps it
  - a suite against the local server needs no AWS account or credential beyond a non-empty placeholder pair
non_goals:
  - transactional rollback, which the store does not offer
  - sharing one table set across parallel tests and cleaning items between them
  - seeding, until requirement:test-data-seeding is extended to this store
related:
  - requirement:parallel-database-tests
  - api:test-run
  - requirement:dynamodb-migration
```
