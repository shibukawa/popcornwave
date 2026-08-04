---
id: requirement:dev-query-runner
type: requirement
title: Development Query Runner
---
A requirement:dev-console pane runs a declared query against the development database through the function flow:sql-generation emitted for it, so a statement is exercised with its own types and its own instrumentation rather than retyped as raw SQL.

```yaml
audience: actor:application-developer
pane_of: requirement:dev-console
mechanism: decision:dev-application-attachment, because the connection a declared statement needs is one only the application process can address
source: every .pw.sql under the data:project-config generate.queries purposes
default: enabled
subjects:
  - each exported statement, with the parameter and result types it declared
  - each unexported statement, reached the way requirement:template-storybook reaches an unexported template
  - the requirement:dynamodb-typed-queries declarations, because data:dynamodb-request-record was shaped so one viewer reads both stores
input:
  form: derived from the declared parameter type, so a typed argument is entered as its type rather than as text spliced into SQL
  absent_parameter: a statement taking none runs with no form at all
output:
  rows: the declared result type, shown as its fields
  exec: the affected count, per data:query-record
  none: sql.one with no row is reported as no row rather than as an error
instrumentation:
  path: the api:instrumented-sql-executor of the running application itself, so a run produces a data:query-record indistinguishable from one a request produced
  explain: a run over the slow_threshold attaches a plan by flow:query-diagnostics, unchanged
  reproduction: the rule:query-reproduction-format snippet is shown with the result, so the run leaves behind something rerunnable outside the pane
  telemetry: the record reaches requirement:dev-telemetry-viewer like the application's own, which is where a run is read back
writes:
  permitted: yes, because a declared statement is what the application would run and half the declarations mutate
  guard: an insert, update, or delete is confirmed before it runs, naming the statement
  scope: whatever database the attached application opened, which is the development one because api:cli-dev is what started it
  recovery: data reseeds through api:cli-seed, which the loop already runs after a migration cycle
  transaction: a run takes no transaction the pane holds open, so the application's pool is never held by a page nobody is looking at
refused:
  arbitrary_sql:
    offered: no
    reason: the pane's value is that it runs the project's own statements with the project's own types, and a free text box is a different tool with a different risk
    alternative: the developer's own client, reached by the DSN policy:startup-summary already reports
non_goals:
  - a schema browser or a table row viewer, which is a different pane and a different question
  - editing a .pw.sql source from the browser
  - saving, naming, or replaying past runs; requirement:dev-telemetry-viewer holds what ran
  - running against anything but the development environment
acceptance:
  - a statement declared with typed parameters is runnable without writing SQL
  - an unexported statement appears in the pane
  - a run emits the same data:query-record an application call would
  - a slow run attaches a plan
  - a mutating statement is not run until it is confirmed
  - a project on an in-process sqlite://:memory: database is served like any other, because the run happens inside the process holding it
  - the pane reports the application as detached while it is down, and recovers on its own when the loop restarts it
  - a binary produced by api:cli-build contains no part of the pane
```
