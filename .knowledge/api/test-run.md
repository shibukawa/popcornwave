---
id: api:test-run
type: api
title: Isolated TestRun
---
testutil.TestRun executes an application from a deep copy of every registered framework and application configuration.

```yaml
package: github.com/shibukawa/popcornwave/testutil
surface:
  - TestRun(t, handler, customize, ...RunOption) *Server
  - Get[T](*Config) T
  - Set[T](*Config, T)
  - Update[T](*Config, func(*T))
  - WithSchemaDir(path)
  - WithTransaction(enabled bool)
isolation:
  source: api:runtime-configuration registered effective values
  copy: deep copy keyed by exact Go type
  mutation: customize changes only the copy
  request: copied framework and user config is installed in data:request-context-capsule
port:
  customize_default: server.port -1
  behavior: reserve an available loopback port before runtime initialization
  effective_copy: replace -1 with the reserved port
database:
  configuration: customize copied data:middleware-runtime-config
  ownership: TestRun opens and closes its own pool
  schema: WithSchemaDir applies api:cli-schema-init lexical SQL transactionally
transaction:
  requirement: requirement:parallel-database-tests
  flow: flow:test-transaction-isolation
  option: WithTransaction(enabled) toggles per-test transaction isolation
  default: off, because a pool sized for one connection or an in-memory database cannot serve both the test transaction and the pool
  on:
    - reject the run when the driver fails rule:savepoint-dialect-support, before opening a pool
    - begin one *sql.Tx and install data:transaction-scope at depth 0
    - the scope belongs to the server resources, so every request context carries it
    - roll back in Server.Close before the pool closes, and fail the test on rollback error
    - a shared database supports t.Parallel across tests
  off:
    - requests use the pooled *sql.DB
    - required for drivers unsupported by rule:savepoint-dialect-support, with test parallelism 1
  scope_note: WithSchemaDir runs before and outside the test transaction
  Server.Context: context carrying the same resources as a request, including the test transaction, for setup and assertions
runtime:
  - validate copied configuration
  - construct production middleware behavior without mutating global config
  - start an HTTP server
  - register testing cleanup
```
