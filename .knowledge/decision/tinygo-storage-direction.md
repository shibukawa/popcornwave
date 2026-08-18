---
id: decision:tinygo-storage-direction
type: decision
title: TinyGo Storage Direction
---
Popcorn Web selects native embedded SQLite as its default relational store, adds PostgreSQL and MySQL once their drivers gained TinyGo TLS, and keeps Redis-compatible servers as first-class shared state.

```yaml
status: accepted
verified: 2026-07-29
selected:
  relational_default: requirement:contrib-sqlite
  relational_also_first_class:
    - requirement:contrib-postgresql
    - requirement:contrib-mysql
  tinygo_native: requirement:contrib-cgosqlite
  network_kv: requirement:contrib-redis-valkey
  network_document: requirement:dynamodb-store, added 2026-07-31 once system:tinygodriver-dynamodb and the system:tinybind dynamo generator made a typed TinyGo path exist
  transport_boundary: decision:local-tls-proxy-boundary
  server_sql_tier: decision:server-sql-support-tier
findings:
  database_sql:
    - TinyGo 0.41.1 imports database/sql and database/sql/driver in the official container
    - pooling, context operations, queries, scanning, and transactions passed a fake-driver smoke test
    - Linux arm64 execution and Linux amd64 cross-compilation passed
  server_sql_drivers:
    superseded_2026_07_17_finding:
      - go-sql-driver/mysql v1.10.0 needed TinyGo patches and could not perform the required TLS upgrade
      - pgx v5.10.0 and lib/pq v1.12.3 required missing TinyGo TLS and net APIs
    current:
      - system:tinygodriver forks both upstreams and replaces only the TLS call with its OS-backed upgrade seam
      - the fork is three patched files out of 145 for pgx, and two build constraints plus the TLS path for mysql
      - both keep upstream DSN syntax, type coverage, transactions, prepared statements, and cancellation
    consequence: decision:server-sql-support-tier promotes both to first class
  embedded_kv:
    - BuntDB v1.3.2 persisted transaction writes and reads under TinyGo Linux arm64 and compiled for amd64
    - bbolt v1.5.0 persisted transaction writes and reads under TinyGo Linux arm64 and compiled for amd64
  network_kv:
    - Redis and Valkey are suitable for shared TTL, counter, session, and small key-value state
    - bounded RESP2 and go-redis v9.17.3 passed Redis 8.4.4 and Valkey 9.1.0 session operations with TinyGo 0.41.1
    - go-redis v9.21.0 and valkey-go v1.0.76 do not compile unchanged with TinyGo 0.41.1
    - first-class transport uses policy:outbound-transport-security instead of requiring direct TinyGo TLS
sqlite_rejections:
  purego_dynamic:
    - ebitengine/purego v0.10.1 loaded libsqlite3 with host Go
    - TinyGo rejected purego CGo constraints and purego depends on standard Go runtime ABI machinery
  wasm_embedded: rejected for runtime overhead and binary-size uncertainty
  modernc_on_tinygo: TinyGo compiler panicked on modernc.org/sqlite v1.54.0 generated code
  mattn_on_tinygo: mattn/go-sqlite3 v1.14.48 used unsupported CGo constraint directives
rationale:
  - native SQLite preserves SQL, transactions, and database/sql without a separate service, which keeps it the scaffold default
  - a pinned amalgamation avoids system SQLite version drift
  - a small dedicated C adapter avoids broad upstream compatibility patches
  - a server engine is now a configuration choice rather than a compatibility question, so requirement:database-engine-selection can offer it at bootstrap
sources:
  tinygo_stdlib: https://tinygo.org/docs/reference/lang-support/stdlib/
  sqlite_c_api: https://sqlite.org/c3ref/intro.html
  purego: https://github.com/ebitengine/purego
  valkey_protocol: https://valkey.io/topics/protocol/
  buntdb: https://github.com/tidwall/buntdb
  bbolt: https://github.com/etcd-io/bbolt
```
