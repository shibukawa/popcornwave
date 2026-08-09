---
id: decision:escape-hatch-transaction-scope
type: decision
title: The Escape Hatch Runs Inside The Open Transaction
---
api:native-connection-access hands the connection the active data:transaction-scope is executing on, not a fresh one from the pool, so work done through the hatch cannot outlive a rollback of the transaction that contained it.

```yaml
status: accepted, implemented 2026-08-09
mechanism:
  fact: the pgx transaction type exposes the connection it runs on
  consequence: >
    handing that connection back gives the callback the same session, so every
    statement it issues is inside the open transaction with no wrapper, no
    second type in the callback signature, and no branch in caller code
  savepoints: >
    a nested api:transaction-runner call is a savepoint on that same
    connection per decision:savepoint-nested-transaction, so the hatch inherits
    the current depth without knowing about it
why_not_a_pooled_connection:
  what: lease a connection from the pool and run the callback there, ignoring
    the open scope
  why_not: >
    the writes would land outside the transaction the caller opened, and a
    rollback of that transaction would leave them standing. Under api:test-run
    they would also escape the depth 0 transaction
    requirement:parallel-database-tests rolls back, so a test would leak rows
    into the next one
  precedent: >
    api:database-selection already refuses the same shape, when SelectDB names
    a writable group other than the scope's, and for the same reason: work
    outside the open transaction reads as atomic and is not
  not_offered_as_an_option: a caller cannot make it safe by knowing about it
callback_must_not_end_the_transaction:
  rule: the callback commits and rolls back nothing, per
    decision:explicit-transaction-boundary
  enforcement: the callback receives the connection, not the transaction, so
    Commit and Rollback are not in reach
  residual: >
    raw COMMIT or ROLLBACK text sent through Exec would still end it, which no
    signature can prevent and which the surface documentation names rather than
    pretends to block
batch_consequence:
  what: a batch sent through the hatch inside a transaction keeps its one round
    trip, because the pgx transport pipelines on the connection either way
  contrast: >
    the sqlbatch transport of rule:batch-engine-capability cannot do this at
    all, since it enters through sql.Conn.Raw and sql.Tx has none. This is the
    sharpest difference between the engines and belongs in the guide
```
