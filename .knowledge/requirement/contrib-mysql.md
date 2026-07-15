---
id: requirement:contrib-mysql
type: requirement
title: TinyGo MySQL Driver
---
contrib/database/mysql implements MySQL protocol clients for common database/sql operations with secure defaults.

```yaml
package: contrib/database/mysql
required:
  - protocol 4.1 capability negotiation
  - TCP connection and TLS upgrade
  - caching_sha2_password over TLS
  - mysql_native_password for legacy interoperability
  - text query protocol
  - binary prepared statement protocol
  - transactions
  - null and core scalar decoding
  - server error code and SQL state extraction
  - connection liveness detection
defaults:
  - TLS required for password authentication outside loopback
  - multi-statements disabled
  - local infile disabled
deferred:
  - protocol compression
  - Unix socket
  - LOAD DATA LOCAL
  - replication protocol
protocol: https://dev.mysql.com/doc/dev/mysql-server/latest/PAGE_PROTOCOL.html
```
