---
id: decision:dev-console-self-sufficiency
type: decision
title: Development Console Self Sufficiency
---
requirement:dev-console answers a development question itself rather than sending the developer to a second tool, when the project's own knowledge is what makes the answer useful and a general tool would not have it.

```yaml
status: accepted
question: whether pw hosts a surface that established tools already provide, such as a database browser beside DBeaver or TablePlus
position: host it when the project's own knowledge is the difference, and link or defer when it is not
what_the_console_knows:
  - which engine the project generates for, and the DSN rule:rdb-dsn-resolution resolved
  - the connection the application actually opened, including an embedded one nothing outside the process can address
  - the statements flow:sql-generation emitted from the project's own .pw.sql sources
  - which tables rule:framework-owned-tables says the application does not own
  - what the loop is doing, so a pane can say why it is unreachable rather than timing out
  - a general tool knows none of this, and is configured with it by hand once per developer per machine
sqlite_is_decisive:
  - requirement:contrib-sqlite is the scaffolded default, so the ordinary project is the one an external tool serves worst
  - an in-process sqlite://:memory: database has no address at all, and no configuration of any external tool reaches it
  - a file-backed one is reachable, and opening it makes a second writer against a pool api:cli-init sizes at one connection
  - so the common case is not "inconvenient with another tool" but "impossible or harmful with one"
applied:
  hosted:
    - requirement:dev-data-pane, because of everything above
    - requirement:dev-query-runner, which has no external equivalent: nothing outside the project can build a declared statement
    - requirement:template-storybook, for the same reason at the template layer
    - requirement:dev-telemetry-viewer, because the loop already receives what it shows
  linked:
    - requirement:dev-api-reference, because the application already serves the renderer and a second one would be a second thing to keep current
  deferred:
    - anything whose value is independent of this project, where a general tool is simply better and the console would be a worse copy of it
not_a_general_tool:
  rule: a hosted surface answers the development question and stops; it does not grow toward the tool it replaced for that question
  excluded: schema editing, migration authoring, connection management, saved workspaces, export formats, and any surface aimed at a database this project did not open
  reason: those are where a general tool's own knowledge is the value, and competing there costs maintenance the framework has no reason to spend
  remedy: a developer wanting them still has their own tool, and the console does not pretend otherwise
cost:
  accepted: pw maintains browser surfaces it could have declined to have
  bounded_by: policy:dev-console-boundary, which keeps every one of them out of an api:cli-build artifact
```
