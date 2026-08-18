---
id: requirement:contrib-mysql
type: requirement
title: MySQL Driver Consumption
---
Popcorn Web consumes the system:tinygodriver MySQL and MariaDB driver, which registers the name mysql and carries a separately licensed TinyGo fork.

```yaml
package: github.com/shibukawa/tinygodriver/database/sql/mysql
support_tier: first_class
driver_name: mysql
servers:
  - MySQL
  - MariaDB
backend:
  standard_go: github.com/go-sql-driver/mysql v1.10.0
  tinygo_or_forced: a fork of that same version carried inside the package
  parity: identical DSN syntax, driver name, and database/sql behavior on both
open: Open(dsn), or sql.Open with the registered driver name
dsn:
  form: go-sql-driver DSN such as user:pass@tcp(host:port)/db, which is not a URL
  consequence: rule:rdb-dsn-resolution strips the scheme prefix before handing the remainder to the driver
  parse_time:
    rule: the engine defaults parseTime=true, and leaves an explicit setting alone
    reason: without it the driver returns DATETIME and TIMESTAMP as bytes, and every scan into a time.Time fails
    evidence: system:goose applied a migration and then could not read its own applied-at column back
    scope: a DSN property, so it is normalized once at the engine rather than in each caller's DSN
tls:
  modes: tls=true, tls=skip-verify, and tls=preferred on both backends
  custom_ca: RegisterTLSConfig takes PEM bytes rather than a crypto/tls value, so one call is portable and the standard-Go backend converts internally
  dialer_conflict: a tls DSN parameter needs the driver's own dialer, and a registered dial function fails with a not-upgradable error instead of connecting in cleartext
  client_certs: unsupported on macOS
  platforms: decision:server-sql-support-tier platform_bounds
licence:
  directory: the TinyGo fork is MPL-2.0 while the rest of system:tinygodriver is Apache-2.0
  effect: file-level copyleft, so redistribution and modification of those files carry MPL-2.0
  obligation: requirement:cli-distribution artifacts and generated projects surface the notice
tinygo_behavior:
  pooling: the descriptor-based liveness check is compiled out under the tinygo tag, because netdev cannot supply a descriptor and every pooled connection was otherwise judged dead
  measured: 200 sequential queries opened 201 server connections before the fix and 0 after, matching standard Go
  timeouts: the DSN timeout parameter has no effect; read timeout, write timeout, and query deadlines work
  transport: IPv4 TCP only; no Unix socket and no IPv6
popcorn_web_scope:
  - pin the tested system:tinygodriver version per requirement:tinygodriver-adoption
  - link the package only where the DSN scheme selects it
  - keep multi-statements and local infile disabled
  - add no wire protocol, pool, or dialect implementation
acceptance:
  - api:rdb-middleware opens, pings, and applies pool bounds through the registered driver
  - requirement:database-migration applies the same migration set under the mysql system:goose dialect
  - requirement:parallel-database-tests savepoint nesting passes, with the rule:savepoint-dialect-support DDL caveat documented
  - flow:query-diagnostics captures a JSON plan per rule:explain-dialect-support
  - requirement:test-data-seeding seeds and asserts through the mysql system:dbtestify dialect
  - credentials never reach logs, errors, config views, or process arguments
non_goals:
  - a Popcorn Web MySQL protocol implementation
  - protocol compression, LOAD DATA LOCAL, or the replication protocol
  - Unix domain sockets or IPv6 under TinyGo
protocol: https://dev.mysql.com/doc/dev/mysql-server/latest/PAGE_PROTOCOL.html
```
