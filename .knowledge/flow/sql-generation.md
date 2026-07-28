---
id: flow:sql-generation
type: flow
title: SQL Generation and Execution
---
Named .pw.sql sources generate typed plans and thin context-based functions over the shared TinyBind SQL runtime.

```yaml
source: .pw.sql
output: "{source-base}_pw_gen.go beside the source"
architecture: decision:tinybind-sql-runtime
generated_function:
  input:
    - context.Context
    - typed query parameters
  executor: selected from the framework request context
  forbidden_input:
    - manual *sql.DB
    - manual *sql.Tx
runtime:
  database_driver: resolved by decision:config-driven-database
  connection: initialized from registered runtime configuration
  outside_transaction: use request database
  inside_transaction: use active transaction
  diagnostics: api:instrumented-sql-executor observes every generated call without changing generated output
cleanup:
  - remove tinybind_shared_pw_gen.go after the corrected system:tinybind generator no longer emits it
  - Popcorn Wave adds no independent SQL classification or response validator
```
