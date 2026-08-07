---
id: requirement:query-diagnostics
type: requirement
title: Database Query Diagnostics
---
A developer can see which SQL ran, how long it took, why a slow statement was slow, and how to rerun it by hand, without editing application code.

```yaml
audience: actor:application-developer
capabilities:
  query_log: one record per executed statement with duration and outcome
  slow_query_explain: plan-only EXPLAIN for statements over a threshold, logged with the record
  reproduction: paste-able parameterized rerun snippet for the same statement
  span: the data:framework-span-set database span, which puts the same statement in the trace beside the render that ran it, and is configured separately because a deployment commonly wants it without a record per statement
configuration: data:query-diagnostics-config
record: data:query-record
behavior: flow:query-diagnostics
seam: api:instrumented-sql-executor
safety: policy:query-log-safety
coverage:
  observed: every generated flow:sql-generation function
  unobserved:
    - framework-internal SQL issued directly against the pool, such as session, auth, and migration statements
    - statements run through a raw transaction handle taken from an api:transaction-runner scope accessor
  reason: decision:executor-seam-instrumentation
  dynamodb:
    observed: every request an application makes through decision:dynamodb-observability-seam, whether from a declared query or an item call
    record: data:dynamodb-request-record, shaped so one viewer reads both stores
    reproduction: rule:dynamodb-reproduction-format
    unobserved: every rule:framework-owned-tables table, per policy:query-log-safety
    no_explain: DynamoDB has no plan to capture; the scanned count of a read is what answers the same question
acceptance:
  - a disabled configuration performs no timing and constructs no wrapper, and neither does a run with the database span off as well
  - a run with the span on and the record off still traces every statement and writes nothing
  - a record names the statement span rather than the request root, so the trace leads to the values and the plan
  - a dev run logs every generated statement with SQL text, duration, outcome, driver, and transaction depth
  - an exec also reports its affected count; a query reports none, per data:query-record
  - a statement over the threshold logs at most one plan for that execution
  - EXPLAIN failure degrades to the plain slow record and never fails the observed call
  - the reproduction snippet reruns the identical prepared statement under rule:query-reproduction-format
  - an unsupported driver disables EXPLAIN without disabling the query log, per rule:explain-dialect-support
  - a non-dev environment emits nothing until explicitly configured, and no bind values until separately enabled
non_goals:
  - query metrics, aggregation, or a slow-query top list
  - automatic index or rewrite suggestions
  - statement-level sampling
  - capturing result rows
```
