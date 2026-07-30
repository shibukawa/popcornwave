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
dialect:
  source: data:project-config project.database, the one place a project states its engine for generation
  passed_to: the system:tinybind generator SQL dialect option
  required: system:tinybind has no default, because a silently assumed dialect emits placeholders the engine rejects at the first query
  carries: the placeholder style today, and whatever later engine difference the generator has to know
  placeholders:
    postgres: $1, $2, and so on
    mysql: "?"
    sqlite: "?"
  naming: the engine name and the system:tinybind dialect agree except for postgres, which tinybind spells postgresql, so the two are mapped rather than assumed equal
  changing_it: rewrites every generated query, so the key and the rule:rdb-dsn-resolution DSN move together
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
