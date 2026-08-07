---
id: requirement:pgx-native-execution
type: requirement
title: pgx Native Execution Path
---
A PostgreSQL connection serves its request-time query path through the pgx-native surface instead of database/sql, initialized by api:rdb-middleware through a rule:rdb-dsn-resolution registry extended so an engine may register a native opener alongside its *sql.DB opener.

```yaml
motivation:
  benchmark: >
    a Popcorn Wave benchmark on the postgres engine showed database/sql lock
    acquisition dominating the query path: the sql.DB pool mutex plus a
    per-conn mutex on every call, and driver.Value boxing per parameter.
    system:tinygodriver measured the same seam at 6 vs 15 allocs and 352 vs
    754 B/op per query, native vs pgxstdlib
  upstream_seams_now_exist:
    driver: system:tinygodriver v1.1.10 database/pgx re-exports the pgx-native
      API on both compilers, with the CancelRequest watcher and fd-carrying
      dialer installed by its ParseConfig; pgxstdlib is now a layer over it
    binding: system:tinybind v0.4.2 sqlbind adds the Rows cursor interface,
      RowsQuerier, UnimplementedQuerier, and the sqlbind.Query dispatch point
      (its sql-driver-agnostic-rows requirement, requested by this project
      2026-08-07), so generated row-returning statements no longer require a
      handle that constructs *sql.Rows
registry_extension:
  contract: >
    database.Engine gains an optional native-open capability next to Open;
    an engine that registers none behaves exactly as today, so sqlite and
    mysql change nothing
  handle: >
    the native opener returns a framework-defined interface, not a pgx type,
    per the requirement:contrib-postgresql non_goal; the interface must cover
    what api:rdb-middleware startup does with a *sql.DB today: pool bounds,
    ping within connect_timeout, close, and executor construction
  resolution_unchanged: scheme selection, dialect derivation, KeepScheme
    handoff, linking by blank import, and rule:dsn-redaction all stay as
    rule:rdb-dsn-resolution records
  registration: the postgres engine package registers both openers from the
    same init, per decision:import-registered-session-plugins
initialization:
  selection: >
    when the resolved engine offers a native opener, api:rdb-middleware opens
    the connection through the bypass; the *sql.DB opener serves engines that
    register nothing else. Selection is automatic with no config opt-out:
    *sql.DB offers pgx nothing at request time, so a fallback option is not
    provided (decided 2026-08-07), and the only *sql.DB uses that remain for
    pgx are the sql_db_consumers_audit tooling cases
  connection_set: >
    data:database-connection-set holds native and *sql.DB connections in one
    set, so a postgres-native default group coexists with a sqlite group;
    group selection, round robin, and the request memo are path-agnostic
  startup_summary: names the execution path per connection next to its label
    and redacted DSN, so the active path is observable rather than inferred
  readiness: the readiness probe pings a native connection through the native
    handle, with the same per-connection unreadiness semantics
pooling:
  strategy: >
    one surface on both compilers: tinygodriver v1.1.11 ships
    database/pgx/pgxpool in the same alias pattern as database/pgx (std binds
    upstream pgx/v5 pgxpool, native binds the vendored fork), so this
    framework imports one path and never imports jackc pgxpool directly. The
    same release renamed pgxstdlib to database/pgx/stdlib, mirroring upstream
  bounds_mapping: >
    MaxOpenConns maps to pgxpool MaxConns, ConnMaxLifetime to
    MaxConnLifetime, ConnMaxIdleTime to MaxConnIdleTime; MaxIdleConns has no
    pgxpool equivalent — the pool prunes idle connections by time, not count —
    and is a documented divergence, not a silent drop
executor_surface:
  interfaces: the native connection's executor implements sqlbind Execer and
    RowsQuerier and embeds UnimplementedQuerier, so it satisfies the
    sqlbind.SQLExecutor contract generated code and the resolvers consume
  dispatch: generated one/optional/many bodies reach it through sqlbind.Query,
    which prefers RowsQuerier; flow:sql-generation output regenerated against
    tinybind v0.4.2 is required once, because pre-seam bodies call
    QueryContext directly and hit the UnimplementedQuerier error
  instrumentation: api:instrumented-sql-executor wraps the same interfaces, so
    flow:query-diagnostics, the query log, and rule:explain-dialect-support
    JSON plan capture work unchanged on the native path per
    decision:executor-seam-instrumentation
  transactions: >
    api:transaction-runner and data:transaction-scope must run on the native
    transaction, including the savepoint nesting rule:savepoint-dialect-support
    and requirement:parallel-database-tests depend on; the scope's *sql.Tx
    ownership becomes an abstraction both paths satisfy
sql_db_consumers_audit:
  stays_database_sql:
    - api:migration-runner and seeding via system:goose and system:dbtestify
      open their own handle through pgx/stdlib from MigrationDSN, so tooling
      is untouched by the bypass
    - system:pw-cli, which links every engine for any project
    - the api:test-run bridge opens a second pgx/stdlib handle beside a native
      connection for schema preparation and the pre-transaction seed; its
      transaction mode runs on the native path since system:dbtestify v0.5.0
      made its Executor driver-agnostic, with the shared test transaction
      reached through the scope's ActiveExecutor
  moved_to_the_executor_seam:
    - sessionstore.NewStore and the Dialect.Columns contract take
      sqlbind.SQLExecutor now, and data:request-context-capsule carries a
      SessionResources.Executor beside the compatibility DB field
    - requirement:dev-data-pane connections run on sqlbind.Query with any-typed
      scans, because sql.RawBytes exists only inside database/sql
  reports_absence:
    - pw.DB(ctx) and pwruntime.Connection.DB have no *sql.DB on a native
      connection; the accessor reports absence rather than fabricating a
      handle, and pwruntime.ConnectionExecutor is the surface that exists on
      every connection
version_pins:
  tinygodriver: ">= v1.1.11, which exports database/pgx/pgxpool on both compilers and renames pgxstdlib to database/pgx/stdlib"
  tinybind: ">= v0.4.2, moving the system:tinybind pin; regeneration of committed *_pw_gen.go accompanies the move per requirement:generated-code-version-tolerance"
verified:
  - contract and runtime integration tests against postgres 17: exec with
    affected counts, the sqlbind cursor, committed and rolled-back native
    transactions, savepoint nesting through the transaction runner, read-only
    refusal, DB absence, and the instrumented executor, all under -race
  - dbseed against postgres 17 on the native executor: a pool-level Apply
    committing per-dataset native transactions, an in-transaction Apply and
    Assert observing uncommitted state and vanishing with the rollback, and a
    mismatch reported as a diff
  - "benchmark on the same parallel SELECT, database/sql vs native: 15 vs 8
    allocs, 812 vs 451 B/op, 34.9 vs 23.0 us/op on a loopback postgres 17"
  - tinygo 0.41.1 -scheduler=threads compiles a program importing the engine
    with its native opener, and host go compiles the same source
acceptance:
  - a postgres connection on host Go serves generated statements with no
    sql.DB pool or per-conn mutex on the query path, and a parallel benchmark
    against the pgxstdlib baseline shows the alloc and contention win
  - transaction runner tests, savepoint nesting, and read-only replica
    selection pass unchanged on the native path
  - query diagnostics captures a JSON plan through the instrumented executor
    on the native path
  - a TinyGo postgres build with -scheduler=threads compiles and serves
    through the same database/pgx/pgxpool surface as host Go
  - a mixed set of one native postgres group and one sqlite group starts,
    serves, and shuts down cleanly
  - sqlite and mysql projects see no behavior change and no new required
    regeneration beyond the tinybind v0.4.2 move itself
  - credentials never reach logs or errors through the new path, per
    rule:dsn-redaction
non_goals:
  - a framework-owned connection pool implementation; the pool is tinygodriver
    database/pgx/pgxpool on both compilers
  - a runtime *sql.DB fallback or opt-out for the pgx engine
  - exposing pgx types in the framework public API; the escape hatch stays
    the requirement:contrib-postgresql Raw path
  - moving migration or seeding off database/sql
  - widening the framework surface with Batch, CopyFrom, or LISTEN/NOTIFY
```
