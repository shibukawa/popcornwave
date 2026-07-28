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
values:
  - bind values are the only path by which application row data enters a framework SQL record
  - disabling them still yields SQL text, duration, outcome, and plan, which is enough for most diagnosis
  - truncate statements and values to the configured bounds before formatting
  - never emit the DSN, its credentials, or connection headers, matching the redaction data:middleware-runtime-config already requires
  - application code owns classification of its own column values, as in data:log-attribute
cost:
  - a disabled configuration does no timing, no allocation, and no wrapper construction
  - an enabled configuration whose severity the logger rejects stops before formatting, so the cost is one timer and one severity check
  - EXPLAIN runs only above slow_threshold and at most once per observed execution
  - diagnostics hold no connection and keep no transaction open beyond the observed statement
  - emission is synchronous on the request goroutine, which is another reason the default is dev only
cardinality:
  - records carry statement text, not a value-derived key, matching requirement:modern-observability safe dimensions
  - the process logger still gates every record, so an enabled query log can be silenced by severity without reconfiguring diagnostics
```
