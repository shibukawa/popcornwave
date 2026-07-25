---
id: decision:savepoint-nested-transaction
type: decision
title: Savepoint-Backed Nested Transaction
---
A nested api:transaction-runner call opens a savepoint inside the active transaction instead of silently joining it, so an inner failure rolls back only inner work.

```yaml
status: accepted
supersedes: transparent reuse of the active *sql.Tx by a nested call
depth_model:
  depth_0: real BEGIN on *sql.DB
  depth_n: SAVEPOINT inside the depth 0 transaction
  state: data:transaction-scope
inner_failure:
  action: ROLLBACK TO SAVEPOINT then RELEASE
  outer_effect: outer transaction stays usable and may still commit
  error: returned to the direct caller, which decides to absorb or propagate
rationale: partial retry and per-step compensation need an inner rollback that does not destroy outer work
unsupported_driver:
  detection: rule:savepoint-dialect-support capability of the resolved driver
  action: nested call returns an error
  no_fallback: never degrade to joining the outer transaction, because callers would lose inner rollback guarantees
poisoning:
  trigger: ROLLBACK TO SAVEPOINT or RELEASE SAVEPOINT fails
  effect: mark data:transaction-scope failed so any outer commit fails instead of committing unknown state
constraints:
  - savepoints are connection scoped, so a scope always owns one *sql.Tx
  - savepoint names are framework generated and never caller supplied
  - no new public API beyond api:transaction-runner
  - isolation level and read-only flags belong to depth 0 only
```
