---
id: policy:query-log-safety
type: policy
title: Query Log Safety
---
requirement:query-diagnostics is a development aid, so its exposure and its cost are both bounded by environment rather than left to the operator to remember.

```yaml
environment:
  dev: query logging and bind values are on by default, per data:query-diagnostics-config auto
  other: both are off by default and require explicit configuration
  visibility: a startup warning names the environment, bind values, and threshold whenever query logging is enabled outside dev
framework_traffic_is_not_recorded:
  rule: a read or write the framework issues against a rule:framework-owned-tables table produces no record at all, in any store
  reason: this is an aid for debugging application code, and the framework's own storage traffic is not application code; a developer reading the log wrote none of it
  covers: the session store of api:session-store, the requirement:contrib-auth-state ceremony records, and any later framework table, on the relational path and on decision:dynamodb-observability-seam alike
  side_effect: the session key hash, the CSRF secret, and the stored payload cannot reach a diagnostic artifact, which policy:session-security forbids and which a captured DynamoDB request body would otherwise have carried in full
  decided_by: table identity, taken before rule:dynamodb-table-naming resolves a deployed name, so a prefix cannot hide a framework table from the exclusion
  what_is_still_visible: a framework store that fails or is slow surfaces as its own error and through ordinary operational metrics, which is where that question is asked anyway
  not_configurable: no setting turns it on, because there is no diagnosis it would serve
span_and_record_split:
  rule: the data:framework-span-set database span carries the statement shape and its timing; the values, the plan, and the rerun snippet stay on data:query-record
  reason: a trace backend is retained longer and read more widely than a log, which is the same reasoning that redacts query string values on the request span
  correlation: the record names the statement span rather than the request root, so a waterfall entry leads to the detail the span does not carry
  not_configurable: no setting puts bind values on a span, because the record is already the place to read them
values:
  - bind values are the only path by which application row data enters a framework SQL record
  - disabling them still yields SQL text, duration, outcome, and plan, which is enough for most diagnosis
  - truncate statements and values to the configured bounds before formatting
  - never emit the DSN, its credentials, or connection headers, matching the redaction data:middleware-runtime-config already requires
  - application code owns classification of its own column values, as in data:log-attribute
cost:
  - a disabled configuration does no timing, no allocation, and no wrapper construction, and the executor is wrapped only when the query log or the database span wants it
  - an enabled configuration whose severity the logger rejects stops before formatting, so the cost is one timer and one severity check
  - EXPLAIN runs only above slow_threshold and at most once per observed execution
  - diagnostics hold no connection and keep no transaction open beyond the observed statement
  - emission is synchronous on the request goroutine, which is another reason the default is dev only
cardinality:
  - records carry statement text, not a value-derived key, matching requirement:modern-observability safe dimensions
  - the process logger still gates every record, so an enabled query log can be silenced by severity without reconfiguring diagnostics
```
