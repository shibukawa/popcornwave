---
id: rule:query-reproduction-format
type: rule
title: Query Reproduction Format
---
The rerun snippet in data:query-record binds parameters and then executes the original statement text, so the reproduced plan matches the observed one.

```yaml
form: a dialect CLI snippet that sets the bind values and then runs the unmodified statement
prohibition:
  literal_substitution: never inline argument values into the SQL
  reason: constant folding and literal-driven index selection can produce a different plan than the parameterized execution being diagnosed
snippet:
  sqlite: parameter-set directives for each placeholder, then the statement text
  postgres: a prepared statement declaration, then execution with the typed argument list
  mysql: user variable assignments, then prepare, execute using, and deallocate
rules:
  - placeholder style matches the generated statement rather than a per-dialect rewrite
  - values render with the quoting of the target CLI and escape control characters and line breaks
  - omit the snippet when bind_values is off, because a parameterized snippet without arguments cannot run
  - omit the snippet when a value was truncated, because a truncated value reproduces a different query
  - the snippet names no DSN, host, database file path, or credential, per policy:query-log-safety
  - an unsupported dialect emits no snippet and leaves sql and args as plain data:query-record fields
  - the snippet is diagnostic output, not an executable the framework ever runs itself
```
