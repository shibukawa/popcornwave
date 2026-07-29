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
  group: data:database-connection-set group the transaction belongs to
  connection: label of the one connection the transaction holds within that group
  depth: int, 0 for the real transaction, incremented per savepoint
  savepoints: active generated names, innermost last
  failed: bool, set when a savepoint or commit operation left state unknown
  dialect: rule:savepoint-dialect-support capability of that connection's driver
  readonly: bool, taken from the selected connection and applied to the depth 0 transaction
naming:
  pattern: pw_sp_{depth}
  properties:
    - ASCII identifier safe on every supported dialect
    - unique within one scope because depth is unique per live savepoint
    - never derived from user input
derived:
  sql_executor: the scope tx while the effective group equals the scope group, otherwise the api:database-selection pool
lifecycle:
  - created by the first depth 0 api:transaction-runner call or by an owner that begins the transaction
  - a bare *sql.Tx installed as a context executor without a scope is adopted into one, so nesting still uses savepoints
  - api:rdb-middleware auto transaction creates it before handler dispatch
  - api:test-run transaction option creates it once per test
  - each nested call pushes one savepoint and pops it on release or rollback
  - discarded after commit or rollback of the depth 0 transaction
constraints:
  - one scope per request context chain, bound to one group for its whole life
  - a group is chosen once at depth 0; no nested call moves a scope to another group
  - callers cannot read, replace, or mutate the scope
  - a scope is not shared across requests, except the one api:test-run scope shared by all requests of a single test
  - concurrent statements on one scope serialize on the single underlying connection
  - savepoint push and pop are sequential per scope; do not nest transactions from concurrent goroutines on one scope
```
