---
id: rule:explain-dialect-support
type: rule
title: EXPLAIN Dialect Support
---
The slow path of flow:query-diagnostics resolves its EXPLAIN statement from the runtime driver name, the same source rule:savepoint-dialect-support uses.

```yaml
mode: plan only
statement:
  sqlite: EXPLAIN QUERY PLAN before the observed statement text
  postgres: EXPLAIN with JSON format before the observed statement text
  mysql: EXPLAIN with JSON format before the observed statement text
default: unsupported
rules:
  - never use ANALYZE, because it executes the observed statement a second time and can repeat its side effects
  - plan-only EXPLAIN is safe for every statement kind, so no read or write filtering is needed
  - pass the original arguments, so the captured plan matches a parameterized execution rather than a literal one
  - an unsupported driver disables EXPLAIN and leaves the rest of flow:query-diagnostics intact
  - report an unsupported driver once at startup through policy:startup-summary, never per statement
  - decision:server-sql-support-tier keeps SQLite the only tier-one dialect, so other mappings activate only with their driver
  - a driver that rejects EXPLAIN for a statement records explain_error and continues
  - a dialect that returns no plan rows for a statement kind records neither a plan nor an error
  - sqlite returns no plan for an INSERT with a VALUES list, including one carrying ON CONFLICT or RETURNING, because the plan describes how rows are found and such a statement finds none
  - sqlite does plan UPDATE, DELETE, and INSERT ... SELECT, so the gap is narrower than write versus read
  - the captured plan is stored as text in data:query-record without framework interpretation
  - a multi-column plan is labeled with its own column names, and a column the dialect names notused is dropped
```
