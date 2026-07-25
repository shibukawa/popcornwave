---
id: decision:tinybind-sql-runtime
type: decision
title: TinyBind SQL Plan Runtime
---
TinyBind owns SQL statement classification, result-contract validation, and the shared execution runtime.

```yaml
status: accepted
owner: system:tinybind
model: mirror the generated plan plus shared runtime separation used by htmlbind
generated:
  - typed parameter and result structures
  - immutable statement plan carrying execution and result metadata
  - thin exported typed function
runtime:
  shared:
    - statement and argument builder
    - Exec and Query execution
    - row cardinality handling
    - row scanning contracts
  framework_boundary: TinyBind calls the framework-configured executor resolver for the active database or transaction
execution:
  sql.exec: Exec
  sql.one: Query requiring exactly one row
  sql.optional: Query allowing zero or one row
  sql.many: Query allowing zero or more rows
generation_validation:
  - row-returning declarations require SELECT or mutation with RETURNING
  - sql.exec rejects SELECT and mutation with RETURNING
  - projected or RETURNING columns must match the declared response contract
  - incompatible or statically unresolved response shapes are generation errors
removed:
  - per-package tinybind_shared_pw_gen.go runtime duplication
  - _tinybindSafeMutation runtime token heuristic
popcorn_wave_scope:
  - consume the corrected TinyBind release
  - retain the existing configured executor resolver
  - regenerate SQL artifacts
  - remove stale generated shared files
  - add no SQL parser, validator, plan runtime, or new public executor API
```
