---
id: requirement:contrib-mysql
type: requirement
title: TinyGo MySQL Driver
---
contrib/database/mysql is a deferred non-first-class investigation and is unsupported by Petitweb releases under decision:server-sql-support-tier.

```yaml
package: contrib/database/mysql
support_tier: non_first_class
compatibility_label: unsupported
status: deferred
blocker: decision:server-sql-support-tier
promotion_requirements:
  - protocol 4.1 capability negotiation
  - TCP connection through decision:local-tls-proxy-boundary
  - caching_sha2_password through a verified TLS upstream proxy
  - mysql_native_password for legacy interoperability
  - text query protocol
  - binary prepared statement protocol
  - transactions
  - null and core scalar decoding
  - server error code and SQL state extraction
  - connection liveness detection
product_boundaries:
  - no scaffold or default dependency
  - no compatibility guarantee
  - no release acceptance gate
defaults:
  - policy:outbound-transport-security required outside loopback
  - multi-statements disabled
  - local infile disabled
deferred:
  - protocol compression
  - Unix socket
  - LOAD DATA LOCAL
  - replication protocol
protocol: https://dev.mysql.com/doc/dev/mysql-server/latest/PAGE_PROTOCOL.html
```
