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
  - WithMigrations(path)
  - WithMigrationsFS(fs.FS)
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
  migration:
    source: WithMigrations reads data:migration-source from disk and WithMigrationsFS reads an embedded tree
    timing: flow:database-migration completes before the server starts
    install: decision:test-migration-snapshot replays a cached data:migration-snapshot into the isolated database
    backend: decision:migration-execution-split selects in-process goose or pw migrate snapshot to produce that artifact
    memory_database: supported on both paths because SQL is transferred instead of a DSN
    fallback: direct apply for a non-sqlite dialect or when the test opts out of snapshots
    default: no schema work when no migration option is supplied
    rollback: never; test databases are created and discarded
runtime:
  - validate copied configuration
  - construct production middleware behavior without mutating global config
  - start an HTTP server
  - register testing cleanup
```
