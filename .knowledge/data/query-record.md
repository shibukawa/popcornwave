---
id: data:query-record
type: data
title: Query Record
---
One executed statement produces one log record whose attributes follow the data:log-attribute scalar contract.

```yaml
message: sql executed
always:
  sql: statement text, truncated to max_sql_length
  duration: observed wall time of the executor call
  outcome: ok or error
  operation: exec or query
conditional:
  driver: resolved runtime driver name, when known
  tx_depth: data:transaction-scope savepoint depth, present only inside a transaction
  args: positional bind values, present only while bind_values is on
  rows_affected: affected count of an exec, absent when the driver cannot report it
  error: safe message when outcome is error
  slow: true when duration reaches slow_threshold
  explain: captured plan when rule:explain-dialect-support supports the driver and the plan has rows
  explain_error: safe message when the plan could not be captured
  reproduction: snippet built by rule:query-reproduction-format
  sql_truncated: true when the statement text hit its bound
  args_truncated: true when any value hit its bound
correlation: request and trace fields come from the api:logger context, not from this record
rules:
  - one record per execution, never per row
  - a query reports no row count, because api:instrumented-sql-executor returns the concrete rows value the caller iterates
  - duration covers the executor call, so for a query it excludes the caller's row scanning, and it always excludes EXPLAIN and formatting
  - args serialize as data:log-attribute scalars; non-scalar values render as a type marker, not a dump
  - user attributes cannot collide with reserved record fields
  - severity is level, or slow_level once slow is true
  - truncated sql and args are marked as truncated so a reader does not treat them as complete
```
