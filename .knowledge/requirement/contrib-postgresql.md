---
id: requirement:contrib-postgresql
type: requirement
title: PostgreSQL Driver Consumption
---
Popcorn Web consumes the system:tinygodriver pgx-based PostgreSQL driver, whose handle comes from a connector rather than from a registered database/sql driver name.

```yaml
package: github.com/shibukawa/tinygodriver/database/pgx/stdlib, renamed from database/sql/pgxstdlib in v1.1.11
native_surface: github.com/shibukawa/tinygodriver/database/pgx since v1.1.10 and database/pgx/pgxpool since v1.1.11, which stdlib now layers on; requirement:pgx-native-execution consumes them for the request-time query path
support_tier: first_class
backend:
  standard_go: upstream github.com/jackc/pgx/v5 stdlib, unmodified
  tinygo_or_forced: vendored pgx with the TLS call rerouted to the https upgrade seam
  parity: one public surface under every build tag combination
open:
  form: Open(dsn) and OpenContext(ctx, dsn) returning *sql.DB
  registration: none; the package registers no driver name and builds its handle from a pgx connector
  consequence: rule:rdb-dsn-resolution binds the postgres scheme to this opener, because sql.Open cannot reach it
  eager_check: OpenContext pings, so configuration errors surface at open instead of at first query
dsn: libpq URL or keyword string, passed through unchanged
tls:
  sslmode: honored on both builds, including verify-full with a custom sslrootcert
  upgrade: the handshake runs on the connected socket after the PostgreSQL SSLRequest
  verify_ca: treated as verify-full, because the native backends cannot skip the host name check
  client_certs: sslcert and sslkey are rejected rather than ignored
  platforms: decision:server-sql-support-tier platform_bounds
cancellation: context cancellation issues a PostgreSQL CancelRequest over a second connection, installed by default on both backends so behavior does not differ by compiler
escape_hatch:
  database_sql: sql.Conn.Raw yields the pgx stdlib connection and then the pgx connection, keeping Batch, CopyFrom, and LISTEN/NOTIFY reachable without widening the framework surface
  native: >
    Raw needs a *sql.DB, which requirement:pgx-native-execution removed from the
    request path, so this hatch now covers only the migration and seeding
    handles; requirement:native-pgx-escape-hatch restores it for a request
inherited: type coverage, prepared statements, transactions, column metadata, and SQLSTATE errors come from pgx on both paths
popcorn_web_scope:
  - pin the tested system:tinygodriver version per requirement:tinygodriver-adoption
  - link the package only where the DSN scheme selects it, so a SQLite project does not carry pgx
  - add no wire protocol, pool, or dialect implementation
acceptance:
  - api:rdb-middleware opens, pings, and applies pool bounds through this opener
  - requirement:database-migration applies the same migration set under the postgres system:goose dialect
  - requirement:parallel-database-tests savepoint nesting passes
  - flow:query-diagnostics captures a JSON plan per rule:explain-dialect-support
  - requirement:test-data-seeding seeds and asserts through the postgres system:dbtestify dialect
  - credentials never reach logs, errors, config views, or process arguments
non_goals:
  - a Popcorn Web PostgreSQL protocol implementation
  - exposing pgx types in the framework public API
  - Unix domain sockets or IPv6 under TinyGo
protocol: https://www.postgresql.org/docs/current/protocol.html
```
