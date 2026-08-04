---
id: requirement:dev-table-viewer
type: requirement
title: Development Table Viewer
---
A requirement:dev-console pane reads the development database's tables and rows through the application's own connection, so a developer can see what a statement did without leaving the console or opening a second client.

```yaml
audience: actor:application-developer
pane_of: requirement:dev-console, shown beside requirement:dev-query-runner because running a statement and looking at what it changed is one activity
mechanism: decision:dev-application-attachment
default: enabled
configuration: data:project-config dev.console.tables
why_the_attachment:
  same_reason_as_the_runner: requirement:contrib-sqlite is embedded, so the connection is a process-local handle; an in-process sqlite://:memory: database has no external existence, and a file-backed one would make a second process a second writer against a pool scaffolded at one connection
  one_path: a server engine could be reached directly, but a pane that read PostgreSQL its own way and SQLite through the application would behave differently per project for no reason a developer can see
  fidelity: reading through the application's connection means the pane sees the session the application configured, rather than a connection a tool opened with its own defaults
shows:
  tables: the tables the live catalog reports, not the ones data:migration-source would produce, because the rows are in the database and a schema that disagrees with them would mislead
  columns: name, declared type, nullability, and key membership
  rows: one bounded page at a time, never a whole table
framework_tables:
  structure: listed, with their columns, because rule:framework-owned-tables names are already in the project's own migration files
  contents: not readable
  reason: policy:query-log-safety keeps framework storage traffic out of every diagnostic artifact so that the session key hash, the CSRF secret, and the stored payload cannot reach one, which policy:session-security forbids; a console page is such an artifact
  presentation: named as excluded with that reason, rather than hidden, because a viewer silently missing tables reads as a broken viewer
read_only:
  rule: the pane issues no insert, update, delete, or DDL
  reason: policy:dev-console-boundary admits an action only when it already exists as a pw subcommand, and editing a row is not one
  contrast: requirement:dev-query-runner may mutate because a declared statement is what the application itself would run; a hand-edited row is what nothing would run
  remedy: a developer who wants to change data writes a declared statement, a seed dataset, or a migration, all of which survive the next reset
safety:
  identifier: a table name is matched against the live catalog before it reaches a statement, so a request carries a selection rather than SQL
  no_predicate: the pane sends no filter, ordering, or expression the developer typed; free text belongs to requirement:dev-query-runner, which runs declared statements, or to a client of their own
  bounds: page size and value length are capped, and a truncated value says so
paging:
  order: the primary key when the table has one
  no_key: the pane pages by offset and says the order is unspecified, rather than implying a stability the database is not promising
  cost: one bounded read per page, holding no transaction open between them, per decision:dev-application-attachment
availability:
  rule: the pane works exactly while the application is up, like every attachment pane
  detached: the pane says the application is detached and recovers when the loop restarts it
non_goals:
  - editing, inserting, or deleting a row
  - arbitrary SQL, a filter box, or a sort control
  - schema editing, which belongs to requirement:database-migration
  - exporting a table
  - reading a production database, or any database api:cli-dev did not start the application against
  - requirement:dynamodb-store items, whose schemaless shape needs a different presentation; the store is worth a pane later and is not this one
acceptance:
  - a project on an in-process sqlite://:memory: database is served like any other, because the read happens inside the process holding it
  - an application table lists its columns and pages its rows
  - a rule:framework-owned-tables table lists its columns and refuses its rows, naming why
  - no request the pane sends carries SQL text
  - a table with no primary key is paged and says the order is unspecified
  - the pane reports the application as detached while it is down
  - a binary produced by api:cli-build contains no part of the pane
```
