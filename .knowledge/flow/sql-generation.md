---
id: flow:sql-generation
type: flow
title: SQL Generation and Execution
---
Named .pw.sql sources generate typed context-based functions that automatically use the active database or transaction executor.

```yaml
source: .pw.sql
output: "{source-base}_pw_gen.go beside the source"
generated_function:
  input:
    - context.Context
    - typed query parameters
  executor: selected from the framework request context
  forbidden_input:
    - manual *sql.DB
    - manual *sql.Tx
runtime:
  database_driver: explicitly imported by the application
  connection: initialized from registered runtime configuration
  outside_transaction: use request database
  inside_transaction: use active transaction
```
