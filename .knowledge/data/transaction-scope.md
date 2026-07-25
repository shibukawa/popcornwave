---
id: data:transaction-scope
type: data
title: Transaction Scope
---
One private per-request value tracking the active transaction and its savepoint nesting for decision:savepoint-nested-transaction.

```yaml
visibility:
  type: private
  context_key: private
  owner: data:request-context-capsule
fields:
  tx: "*sql.Tx opened at depth 0"
  depth: int, 0 for the real transaction, incremented per savepoint
  savepoints: active generated names, innermost last
  failed: bool, set when a savepoint or commit operation left state unknown
  dialect: rule:savepoint-dialect-support capability of the resolved driver
naming:
  pattern: pw_sp_{depth}
  properties:
    - ASCII identifier safe on every supported dialect
    - unique within one scope because depth is unique per live savepoint
    - never derived from user input
derived:
  sql_executor: always the scope tx while a scope exists, otherwise *sql.DB
lifecycle:
  - created by the first depth 0 api:transaction-runner call or by an owner that begins the transaction
  - a bare *sql.Tx installed as a context executor without a scope is adopted into one, so nesting still uses savepoints
  - api:rdb-middleware auto transaction creates it before handler dispatch
  - api:test-run transaction option creates it once per test
  - each nested call pushes one savepoint and pops it on release or rollback
  - discarded after commit or rollback of the depth 0 transaction
constraints:
  - one scope per request context chain
  - callers cannot read, replace, or mutate the scope
  - a scope is not shared across requests, except the one api:test-run scope shared by all requests of a single test
  - concurrent statements on one scope serialize on the single underlying connection
  - savepoint push and pop are sequential per scope; do not nest transactions from concurrent goroutines on one scope
```
