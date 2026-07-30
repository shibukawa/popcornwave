---
id: api:instrumented-sql-executor
type: api
title: Instrumented SQL Executor
---
The framework executor resolver returns an observing wrapper so requirement:query-diagnostics needs no change to generated code, drivers, or call sites.

```yaml
resolution:
  disabled: return the underlying executor unchanged
  enabled: return a wrapper over the same executor, built once per resolution rather than per statement
surface:
  - the wrapper implements the same exec and query context methods as the executor it wraps
  - it delegates the call verbatim and rewrites no SQL and no arguments
  - it returns the observed error unmodified
  - it exposes the wrapped value for unwrapping
unwrap:
  - api:transaction-runner scope adoption unwraps before asserting a concrete transaction type
  - accessors that hand out a raw transaction handle return the unwrapped value
  - api:test-run keeps working because the wrapper sits above the scope executor, not inside it
explain:
  executor: the observed executor itself, so the plan sees the same transaction and snapshot
  isolation: EXPLAIN failure is recorded in data:query-record and never surfaces to the caller
rules:
  - the wrapper observes only what the executor receives, so no statement name or source location is available
  - the query method returns the concrete rows value, which cannot be decorated, so row count and scan time stay outside the measurement
  - diagnostics work never changes the observed call result, its error, or its row iteration
  - selection happens at resolution time, per decision:executor-seam-instrumentation
  - api:rdb-middleware still owns the pool and installs the executor; this layer only decorates it
```
