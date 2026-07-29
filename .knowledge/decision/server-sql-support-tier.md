---
id: decision:server-sql-support-tier
type: decision
title: Server SQL Support Tier
---
PostgreSQL and MySQL are first-class alongside SQLite, because system:tinygodriver ships drivers that build under TinyGo and upgrade TLS in band on the already-connected socket.

```yaml
status: accepted
verified: 2026-07-29
supersedes: the exclusion that named driver and protocol compatibility the blocker
first_class:
  - requirement:contrib-sqlite
  - requirement:contrib-postgresql
  - requirement:contrib-mysql
default_engine: requirement:contrib-sqlite, because a scaffolded project runs with no server to start
resolved_blocker:
  was: TinyGo ships crypto/tls as a stub, so no maintained driver could upgrade a connected socket
  now: both drivers route the handshake through the system:tinygodriver https upgrade seam over the OS TLS stack
  shape: each driver is a fork of its maintained upstream with the TLS call replaced, not a new wire implementation
  release: system:tinygodriver v1.1.0
platform_bounds:
  standard_go: crypto/tls on every OS, client certificates included
  tinygo_linux: mbedTLS, TLS 1.3, client certificates supported
  tinygo_darwin: Secure Transport, TLS 1.2 maximum on the upgrade path, no client certificates; -tags darwinstarttlswith13 selects mbedTLS and TLS 1.3
  tinygo_windows: Schannel, implemented and build-verified but never executed on Windows, so it stays unverified rather than supported
  every_tinygo_target:
    - IPv4 TCP only; no Unix domain socket and no IPv6
    - "-scheduler=threads required per rule:tinygo-runtime-compatibility"
  never: silent plaintext fallback; a platform without a TLS backend refuses any mode but disable
promoted_effects:
  - api:cli-init and api:cli-add offer an engine choice per requirement:database-engine-selection
  - rule:rdb-dsn-resolution resolves all three schemes and no longer assumes a registered driver name
  - system:goose treats sqlite3, postgres, and mysql as supported dialects
  - rule:savepoint-dialect-support and rule:explain-dialect-support mappings activate for the selected driver
  - release acceptance runs the shared SQL contract fixtures on all three engines
  - documentation may claim runtime interoperability inside platform_bounds
retained_bounds:
  - decision:local-tls-proxy-boundary stays a supported deployment and is no longer a prerequisite
  - policy:outbound-transport-security still governs every hop that leaves the local workload
  - a .pw.sql source and a migration are written for one dialect, and the framework does not translate between them
  - an unverified platform is documented as unverified rather than claimed
demotion_triggers:
  - an upstream bump the vendored fork cannot follow without unsafe patches
  - a supported platform losing its TLS upgrade backend
  - contract fixtures that cannot pass on an engine without dialect-specific application code
```
