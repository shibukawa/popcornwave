---
id: requirement:dev-query-runner
type: requirement
title: Development Query Runner
---
The requirement:dev-data-pane runs a declared statement with supplied parameters, and runs a statement the developer wrote, so a query can be tried and its effect inspected without routing a request that reaches it.

```yaml
audience: actor:application-developer
pane_of: requirement:dev-data-pane, sharing its attachment and its page
mechanism: decision:dev-application-attachment
default: enabled
configuration: data:project-config dev.console.queries
declared:
  source: every .pw.sql under the data:project-config generate.queries purposes
  subjects:
    - each exported statement, with the parameters its generated builder declares
    - each unexported statement, reached the way requirement:template-storybook reaches an unexported template
  form: a field per declared parameter, so a statement is tried without writing SQL
  built_by: the generated builder, so the pane runs the statement the application would run rather than a second rendering of it
  conditional_sql: a statement whose text is assembled from its arguments is assembled the same way here, because the builder is what assembles it
  shown: the built SQL beside the result, since that is the thing a developer is checking
  registration:
    shape: a pwdev-constrained file generated into the queries package, so an unexported builder registers from where it is reachable
    same_technique: decision:dev-harness-process, which does this for templates
statement_console:
  offered: yes
  supersedes: the earlier refusal, which held that a free text box was a different tool with a different risk
  why_reversed: requirement:dev-data-pane edits rows, so a developer who can already change data gains nothing from being denied the text box, while losing the investigation the pane exists for
  bounded: the number of rows returned, not what may be typed
  classification:
    rule: whether a statement returns rows is decided from its text before it runs, never by trying one call and falling back to the other
    reason: SQLite accepts an UPDATE through the query call without complaint, so a fallback would report nothing for a write that already ran, and then run it a second time
    returns_rows: SELECT, WITH, VALUES, TABLE, SHOW, EXPLAIN, PRAGMA, DESCRIBE, and any statement with a RETURNING clause
    returning: matched as a word, so a column named for it does not change how its statement runs
instrumentation:
  path: the api:instrumented-sql-executor of the running application, so a declared run produces a data:query-record indistinguishable from one a request produced
  explain: a slow run attaches a plan through flow:query-diagnostics, unchanged
  telemetry: the record reaches requirement:dev-telemetry-viewer like the application's own, which is where a run is read back
writes:
  permitted: yes, for both halves
  scope: whatever database the attached application opened, which is the development one because api:cli-dev started it
  recovery: data reseeds through api:cli-seed, which the loop already runs after a migration cycle
  transaction: no run holds one open, so the application's pool is never held by a page nobody is looking at
non_goals:
  - editing a .pw.sql source from the browser
  - saving, naming, or replaying past runs; requirement:dev-telemetry-viewer holds what ran
  - running against anything but the database the running application opened
  - a query builder, a schema designer, or anything aimed at authoring rather than trying
acceptance:
  - a declared statement with typed parameters is runnable without writing SQL
  - an unexported declared statement appears in the pane
  - the SQL shown is the one the generated builder produced
  - a declared run emits the same data:query-record an application call would
  - a written statement returning rows shows them; one that writes reports its affected count
  - a written UPDATE is executed exactly once and reports what it changed
  - a failing statement reports the engine's own error
  - a binary produced by api:cli-build contains no part of the pane
```
