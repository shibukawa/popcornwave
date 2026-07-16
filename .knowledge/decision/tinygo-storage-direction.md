---
id: decision:tinygo-storage-direction
type: decision
title: TinyGo Storage Direction
---
Petitweb selects native embedded SQLite as its first relational store while retaining key-value stores as separate future integrations.

```yaml
status: accepted
verified: 2026-07-17
selected:
  relational: requirement:contrib-sqlite
  tinygo_native: requirement:contrib-cgosqlite
  server_sql_tier: decision:server-sql-support-tier
findings:
  database_sql:
    - TinyGo 0.41.1 imports database/sql and database/sql/driver in the official container
    - pooling, context operations, queries, scanning, and transactions passed a fake-driver smoke test
    - Linux arm64 execution and Linux amd64 cross-compilation passed
  server_sql_drivers:
    - go-sql-driver/mysql v1.10.0 needs TinyGo patches and cannot perform required TLS upgrade
    - pgx v5.10.0 and lib/pq v1.12.3 require missing TinyGo TLS and net APIs
  embedded_kv:
    - BuntDB v1.3.2 persisted transaction writes and reads under TinyGo Linux arm64 and compiled for amd64
    - bbolt v1.5.0 persisted transaction writes and reads under TinyGo Linux arm64 and compiled for amd64
  network_kv:
    - Valkey remains suitable for shared TTL, counter, and session state
    - valkey-go v1.0.76 and go-redis v9.21.0 do not compile unchanged with TinyGo 0.41.1
    - a bounded RESP2 client remains a feasible future package
sqlite_rejections:
  purego_dynamic:
    - ebitengine/purego v0.10.1 loaded libsqlite3 with host Go
    - TinyGo rejected purego CGo constraints and purego depends on standard Go runtime ABI machinery
  wasm_embedded: rejected for runtime overhead and binary-size uncertainty
  modernc_on_tinygo: TinyGo compiler panicked on modernc.org/sqlite v1.54.0 generated code
  mattn_on_tinygo: mattn/go-sqlite3 v1.14.48 used unsupported CGo constraint directives
rationale:
  - native SQLite preserves SQL, transactions, and database/sql without a separate service
  - a pinned amalgamation avoids system SQLite version drift
  - a small dedicated C adapter avoids broad upstream compatibility patches
sources:
  tinygo_stdlib: https://tinygo.org/docs/reference/lang-support/stdlib/
  sqlite_c_api: https://sqlite.org/c3ref/intro.html
  purego: https://github.com/ebitengine/purego
  valkey_protocol: https://valkey.io/topics/protocol/
  buntdb: https://github.com/tidwall/buntdb
  bbolt: https://github.com/etcd-io/bbolt
```
