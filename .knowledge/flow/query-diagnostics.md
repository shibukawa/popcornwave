---
id: flow:query-diagnostics
type: flow
title: Query Diagnostics Emission
---
Each observed statement runs one measure, classify, explain, and emit pass around the unchanged executor call.

```yaml
trigger: a generated flow:sql-generation function resolves its executor
steps:
  - resolve data:query-diagnostics-config once; return the bare executor when disabled
  - start the timer and delegate to api:instrumented-sql-executor
  - stop the timer when the executor call returns
  - compare duration with slow_threshold to set slow, and pick level or slow_level from it
  - stop before any formatting when the logger is not enabled for that severity
  - build data:query-record from SQL text, outcome, duration, affected count, transaction depth, and driver
  - attach bind values when bind_values is on
  - stop here when not slow, and emit
slow_path:
  - resolve the EXPLAIN statement through rule:explain-dialect-support
  - skip it when the observed context is already done, because the plan would need a context this flow does not own
  - run it on the observed executor with the original arguments
  - attach the plan, attach explain_error, or attach neither when the dialect returns no plan rows
  - build the rerun snippet through rule:query-reproduction-format when reproduction is on and every value survived intact
  - emit at slow_level
emit: api:logger, then policy:log-emission
failure:
  observed_call: propagates unchanged; a failed statement still emits its record
  explain: captured in the record only
rules:
  - the observed call is never retried, buffered, or re-executed
  - EXPLAIN runs at most once per observed execution
  - a failed slow statement still gets an EXPLAIN attempt, because the plan often explains the failure
  - policy:query-log-safety governs what may appear in the emitted record
```
