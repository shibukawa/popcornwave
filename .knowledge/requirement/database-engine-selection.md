---
id: requirement:database-engine-selection
type: requirement
title: Database Engine Selection
---
A project picks its database engine while it is being created, because the DSN, the development server, the migration dialect, and the starter SQL all follow from that answer.

```yaml
motivation:
  - decision:server-sql-support-tier makes three engines first-class, so the scaffold can no longer assume one
  - an engine changed after the fact costs a DSN rewrite, a dialect rewrite of every migration and .pw.sql source, and a new development server
  - a project that targets PostgreSQL should not start on SQLite and be migrated by hand on day two
scope:
  init: api:cli-init engine question
  add: api:cli-add database capability
  resolution: rule:rdb-dsn-resolution
  interaction: decision:interactive-project-bootstrap
question:
  step: conditional, asked only when the database answer is yes
  position: directly after the database question and before authentication, so a dependent question follows the one it depends on
  choices:
    - sqlite
    - postgres
    - mysql
  default: sqlite, because the scaffolded project then runs with no server to start
  shortcut: "--db=sqlite|postgres|mysql, which conflicts with --no-database"
  skipped_effect: a declined database never applies an engine, per the decision:interactive-project-bootstrap conditional-step rule
per_engine_scaffold:
  sqlite:
    dsn: sqlite://{project}.db
    development_server: none
  postgres:
    dsn: postgres://{project}:{project}@127.0.0.1:5432/{project}?sslmode=disable
    development_server: the PostgreSQL package in devbox.json
  mysql:
    dsn: mysql://{project}:{project}@tcp(127.0.0.1:3306)/{project}
    development_server: the MySQL package in devbox.json
common_to_every_engine:
  - one data:middleware-runtime-config rdb section, differing in the DSN and in pool bounds sized for the engine
  - data:project-config project.database naming the engine, which is what generation reads
  - the starter migration, written in the selected dialect
  - the starter .pw.sql example, generated for the selected dialect
  - the rule:framework-owned-tables migrations, in the dialect the owning package publishes for it
  - a blank import of the rule:rdb-dsn-resolution engine package in the entry point, for an engine pw does not link itself
  - data:project-config project.toolchain unchanged, because the engine does not change the compiler
generated_sql:
  recorded_in: data:project-config project.database, written by the same answer
  read_by: api:cli-generate, which passes it to system:tinybind
  effect: every engine has a working generated SQL path, so the .pw.sql example is scaffolded for all three
  detail: flow:sql-generation
development_server:
  model: the requirement:contrib-redis-valkey model, a package in devbox.json that api:cli-dev starts
  reason: one mechanism already exists for a development service, so a second one would have to be kept in agreement with it
  without_devbox: the DSN is still written, and the server the operator must supply is printed as a follow-up
  credentials: development-only values in config.dev.toml, never reused as a deployment default
  transport: the scaffolded DSN is loopback plaintext, which policy:outbound-transport-security permits inside the local hop
tinygo_interaction:
  scheduler: a TinyGo project on a server engine builds with -scheduler=threads per rule:tinygo-runtime-compatibility
  netdev: the existing tinygohelper.go blank import already satisfies the driver's network requirement
acceptance:
  - each engine produces a project where pw dev applies migrations and serves the starter page without hand edits
  - the DSN, the schema dialect, the development server package, the driver import, and the generation dialect all follow the one answer
  - a generated query runs unchanged against the engine the same answer selected
  - the recorded engine and the rdb DSN scheme name the same engine
  - the engine question is absent from the wizard, the step counter, and the review screen when the database is declined
  - "--db with --no-database fails before anything is written"
  - api:cli-add database reaches the same file state for the same engine as api:cli-init
  - a project without the Devbox environment still receives a valid DSN and a printed server requirement
  - no scaffolded credential appears in a file the project would deploy
non_goals:
  - converting an existing project from one engine to another
  - one .pw.sql source that compiles for every dialect at once; a project picks its engine and generates for it
  - more than one database in a project
  - provisioning a managed or remote database
```
