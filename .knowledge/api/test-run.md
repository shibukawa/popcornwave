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
runtime:
  - validate copied configuration
  - construct production middleware behavior without mutating global config
  - start an HTTP server
  - register testing cleanup
```
