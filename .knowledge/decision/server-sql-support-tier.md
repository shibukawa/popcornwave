---
id: decision:server-sql-support-tier
type: decision
title: Server SQL Support Tier
---
Popcorn Wave excludes PostgreSQL and MySQL from first-class support until compatible maintained drivers pass through the selected local TLS proxy boundary; direct TinyGo TLS is not a promotion gate.

```yaml
status: accepted
first_class:
  - requirement:contrib-sqlite
non_first_class:
  - requirement:contrib-postgresql
  - requirement:contrib-mysql
compatibility_label: unsupported
evidence:
  shared_sql:
    - database/sql and database/sql/driver work in the tested TinyGo 0.41.1 official container
    - the remaining blocker is driver and protocol compatibility rather than database/sql or direct TLS
  tinygo_tls:
    - required tls.Config.Clone, tls.Conn, certificate, resolver, and connection-state APIs are incomplete or absent
    - tls.Client for in-place protocol TLS upgrade is unimplemented in TinyGo 0.41.1
  mysql:
    - go-sql-driver/mysql v1.10.0 fails unchanged on tls.Config.Clone
    - a minimal clone patch compiles but cannot make the required in-place TLS upgrade safely
  postgresql:
    - pgx v5.10.0 fails on TLS and net resolver APIs
    - lib/pq v1.12.3 fails on TLS clone, certificate, renegotiation, and connection-state APIs
product_effect:
  - project generators and examples do not scaffold PostgreSQL or MySQL
  - framework documentation and build checks do not claim PostgreSQL or MySQL runtime interoperability
  - release acceptance does not depend on either server database
  - documentation may describe decision:local-tls-proxy-boundary as the required secure transport path without claiming driver support
  - applications may experiment with pgBouncer, ProxySQL, HAProxy, or stunnel without Popcorn Wave driver compatibility guarantees
promotion_gates:
  - maintained driver compiles without unsafe compatibility patches on supported TinyGo targets
  - plaintext protocol interoperability passes against an approved same-workload proxy endpoint
  - policy:outbound-transport-security passes verified-TLS upstream tests
  - authentication, prepared statement, transaction, cancellation, and reconnect fixtures pass
  - live interoperability passes against declared server versions
  - binary size and memory remain within contrib policy
```
