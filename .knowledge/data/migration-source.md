---
id: data:migration-source
type: data
title: Migration Source Tree
---
Migrations are version-control-owned SQL files in one project directory, addressed as an fs.FS so on-disk and embedded trees behave identically.

```yaml
default_directory: migrations
configuration: data:project-config migration.dir
file_name: "{version}_{name}.sql"
version: zero-padded numeric prefix ordered lexically and numerically
content:
  format: goose annotated SQL
  markers:
    - "-- +goose Up"
    - "-- +goose Down"
    - "-- +goose StatementBegin / StatementEnd for multi-statement bodies"
    - "-- +goose NO TRANSACTION for dialects or statements that reject transactional DDL"
  rule: every migration declares a Down section because api:cli-migrate publishes down and down-to
scaffolded_version_1:
  body: comments only, in the dialect of the selected engine
  shows: the goose markers, the dialect the project will write in, and a commented-out table and its Down, ready to uncomment
  creates: nothing
  defect_this_fixes: the starter migration created a users table that no scaffolded handler, page, or query the reader keeps ever reads, so selecting a database silently added a schema the operator did not ask for and has to delete before writing their own version 1
  paired_query: the queries/users.pw.sql example is commented out with it, since a live statement against a table that no longer exists is worse than no example
  cost: a fresh project no longer demonstrates typed SQL until two files are uncommented, which the files themselves say
  still_applied: api:cli-migrate applies it and records version 1, so the version sequence a later migration continues from is unchanged
state:
  table: goose_db_version
  owner: system:goose
  rules:
    - the version table is engine-owned and never referenced by application SQL
    - flow:sql-generation excludes the version table and the migration directory
access:
  disk: system:pw-cli and the development loop read the directory
  embedded: an application may supply an embed.FS of the same tree
  requirement: both forms resolve to identical version ordering and content
rules:
  - files are handwritten and committed
  - an applied file is immutable; corrections ship as a new version
  - Go function migrations are rejected under decision:goose-migration-engine
  - the directory is watched by api:cli-dev and excluded from Go rebuild inputs
  - this directory is the only framework-owned schema source convention
  - a file here carries schema and not sample rows, per policy:migration-safety content_boundary
replaces:
  removed_convention: the dbschema directory of unversioned lexical initialization SQL
  reason: requirement:database-migration owns initial schema as version 1
```
