---
id: decision:executor-seam-instrumentation
type: decision
title: Executor Seam Instrumentation
---
Query diagnostics attach at the framework executor resolver that every generated SQL function already calls, not at the driver and not in the generator.

```yaml
status: accepted
seam: api:instrumented-sql-executor
rejected:
  driver_wrapper:
    - a registered wrapping driver loses the request context, so records could not correlate with api:logger or transaction depth
    - it would also capture framework-internal SQL, widening the surface beyond a development aid
    - it forces every requirement:contrib-database driver to stay wrapper-compatible
  codegen_change:
    - statement names would become available, but every project would have to regenerate to gain diagnostics
    - the change lands in system:tinybind rather than in the framework
consequences:
  - coverage equals generated SQL, so requirement:query-diagnostics states its unobserved set explicitly
  - data:query-record carries no statement name or source location, because the executor sees only SQL text and arguments
  - api:transaction-runner scope adoption must unwrap before asserting a concrete transaction type
  - accessors that expose a raw transaction handle return the unwrapped executor
  - no change is required in flow:sql-generation output or in decision:tinybind-sql-runtime
  - a later statement-name field stays possible without moving the seam, once the generator passes one through
```
