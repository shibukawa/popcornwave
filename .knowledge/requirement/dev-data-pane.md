---
id: requirement:dev-data-pane
type: requirement
title: Development Data Pane
---
A requirement:dev-console pane browses and edits the development database through the application's own connection, so investigating what the code did and fixing what it left behind happen without a second tool and without leaving the console.

```yaml
audience: actor:application-developer
supersedes: the read-only table viewer this began as, whose refusal to edit sent the developer to the tool this pane exists to replace
why_hosted: decision:dev-console-self-sufficiency
mechanism: decision:dev-application-attachment
default: enabled
configuration: data:project-config dev.console.data
halves:
  browsing: schema and rows, described here
  running: requirement:dev-query-runner, on the same pane and the same attachment, because running a statement and looking at what it changed is one activity
engines:
  first_class: requirement:contrib-sqlite, requirement:contrib-postgresql, requirement:contrib-mysql, per decision:server-sql-support-tier
  differences: identifier quoting, placeholder form, and where the catalog lives
  shape: one table of per-engine spellings rather than a branch per query, so adding an engine is one entry
  sqlite: reads a pragma where the server engines read information_schema
  postgres: takes the primary key from the constraint, because information_schema carries no per-column key flag there
shows:
  tables: what the live catalog reports, not what data:migration-source would produce, because the rows are in the database and a schema disagreeing with them would mislead
  columns: name, declared type, nullability, and position in the primary key
  rows: one bounded page at a time, never a whole table, because holding the application's pool for the length of its largest table is the one thing a development tool must not do to the process it observes
  null: distinct from the empty string, which is most of why a developer opens this
edits:
  operations: update, insert, and delete, one row at a time
  address: the whole primary key, in key order
  no_key: refused with the reason, rather than an edit that would change an unknown number of rows
  values: bind parameters throughout; only column names reach the statement text, and only after the catalog has confirmed them
  reporting: the affected count, including zero, because a row that moved or was already gone is the case a silent success hides
framework_tables:
  shown: yes, and readable
  marked: yes, by the rule:framework-owned-tables prefix
  why_not_blocked: requirement:dev-query-runner accepts a statement, so the rows are reachable regardless and blocking them here would be theatre rather than a boundary
  why_marked: a row in one of these was written by code the developer did not write, and reading it as application data is the mistake worth preventing
paging:
  order: the primary key where there is one
  no_key: paged by offset, with the order reported as unspecified rather than implying a stability the engine is not promising
  cost: one bounded read per page, holding no transaction between them
availability:
  rule: the pane works exactly while the application is up, because the connection it uses belongs to that process
  detached: the pane says the application is not attached and recovers when the loop restarts it
non_goals:
  - schema editing, migration authoring, and connection management, per decision:dev-console-self-sufficiency
  - reading any database but the one the running application opened
  - filtering, sorting, or searching from the grid; requirement:dev-query-runner is where an expression belongs
  - exporting a table
  - requirement:dynamodb-store items, whose schemaless shape needs a different presentation and is a pane of its own later
acceptance:
  - a project on an in-process sqlite://:memory: database is served like any other, because the read happens inside the process holding it
  - each of the three engines lists its tables and describes its columns
  - a table lists its columns and pages its rows, with NULL distinguishable from an empty string
  - a row is updated, inserted, and deleted by primary key, and the affected count is reported
  - a table with no primary key is paged, says its order is unspecified, and refuses a row edit with the reason
  - a name the catalog does not report is refused rather than quoted into a statement
  - a rule:framework-owned-tables table is readable and marked
  - the pane reports the application as detached while it is down
  - a binary produced by api:cli-build contains no part of the pane
```
