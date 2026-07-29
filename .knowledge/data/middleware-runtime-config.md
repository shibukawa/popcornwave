---
id: data:middleware-runtime-config
type: data
title: Middleware Runtime Config
---
The `middleware` binding selects and orders the Popcorn Wave middleware applied by Run and Middlewares.

```yaml
switches:
  - recovery
  - trusted proxy
  - request ID
  - OpenTelemetry root span
  - access log
  - request size and timeout limits
  - session and authentication
  - CORS
  - response compression
rdb_fields:
  rdb.enabled: bool
  rdb.dsn: scheme://rest resolved by rule:rdb-dsn-resolution, such as sqlite://app.db, sqlite://:memory:, postgres://user:pass@host:5432/db?sslmode=verify-full, or mysql://user:pass@tcp(host:3306)/db
  rdb.connect_timeout: duration
  rdb.max_open_conns: non-negative integer
  rdb.max_idle_conns: non-negative integer
  rdb.conn_max_lifetime: non-negative duration
  rdb.conn_max_idle_time: non-negative duration
recommended_order:
  - recovery
  - trusted proxy
  - request ID
  - root span
  - access log
  - limits and timeout
  - database pool
  - session and authentication
  - security.csrf and CORS
  - compression
  - application handler
rules:
  - policy:web-middleware defines interoperability with user middleware
  - the effective middleware set and order come from validated framework configuration
  - reject orders that violate required security or resource dependencies
  - enabled middleware validates its required config binding and linked implementation at startup
  - access logging and root-span middleware share data:observability-runtime-config
  - session middleware uses data:session-runtime-config and flow:session-lifecycle
  - CSRF and response-header middleware use data:security-runtime-config
  - enabled rdb middleware uses api:rdb-middleware
  - rdb DSN scheme resolves an opener and a dialect through rule:rdb-dsn-resolution, not a driver name
  - apply pool fields through database/sql without driver-specific assumptions
  - zero pool counts and durations retain database/sql zero-value semantics
  - max_idle_conns cannot exceed a positive max_open_conns
  - sqlite://:memory: requires max_open_conns 1 unless an explicitly supported shared-memory DSN is used
  - a server engine needs pool bounds sized against its own connection limit, which sqlite guidance does not imply
  - reject an unknown or unlinked DSN scheme, malformed DSN, invalid pool bounds, or failed startup ping
  - redact DSN credentials and sensitive query values from config views, logs, and errors
  - disabling a dependency of another enabled feature is a startup error
```
