---
id: system:tinygodriver
type: system
title: tinygodriver
---
tinygodriver is the external owner of reusable TinyGo compatibility packages consumed by Popcorn Wave.

```yaml
module: github.com/shibukawa/tinygodriver
source: https://github.com/shibukawa/tinygodriver
packages:
  netdev: github.com/shibukawa/tinygodriver/netdev
  https: github.com/shibukawa/tinygodriver/https
  httpmux: github.com/shibukawa/tinygodriver/httpmux
  httprevproxy: github.com/shibukawa/tinygodriver/httprevproxy
  zstd: github.com/shibukawa/tinygodriver/compress/zstd
  sqlite: github.com/shibukawa/tinygodriver/database/sql/sqlite
  postgresql: github.com/shibukawa/tinygodriver/database/pgx/stdlib, renamed from database/sql/pgxstdlib in v1.1.11, plus database/pgx/pgxpool for the native pool
  mysql: github.com/shibukawa/tinygodriver/database/sql/mysql
  sqlbatch: github.com/shibukawa/tinygodriver/database/sql/sqlbatch
  dynamodb: github.com/shibukawa/tinygodriver/nosql/dynamodb
  datastore: github.com/shibukawa/tinygodriver/nosql/datastore
  google: github.com/shibukawa/tinygodriver/cloud/google
roles:
  netdev: host TCP/IP Netdever registration for TinyGo
  https: net/http-compatible HTTPS client over the OS TLS stack, exposing the in-band upgrade seam the database drivers use
  httpmux: Go 1.22+ ServeMux-compatible routing for TinyGo
  httprevproxy: TinyGo-compatible net/http/httputil.ReverseProxy subset
  zstd: bounded TinyGo encoder with optimized host fallback, streaming-capable from v1.0.4
  sqlite: portable database/sql SQLite facade selecting a host or TinyGo backend
  postgresql: pgx stdlib driver, vendored with TLS rerouted for TinyGo, from v1.0.6
  mysql: MySQL and MariaDB driver forked from go-sql-driver for TinyGo, from v1.1.0
  sqlbatch: batched statement execution over a *sql.DB, one queue shape with a transport each driver package registers for itself; reached directly by a caller rather than wrapped, per rule:batch-engine-capability
  dynamodb: DynamoDB JSON-protocol client written to build under TinyGo, from v1.1.3; detailed in system:tinygodriver-dynamodb
  datastore: Firestore in Datastore mode over the Datastore v1 JSON API, from v1.1.4 and depended on from v1.1.9; detailed in system:tinygodriver-firestore
  google: Google Cloud credentials and bearer tokens, with the RSA signing split out so a token-only or metadata-only build links none of it; what the datastore client authenticates with
standard_go:
  netdev: no-op registration
  https: crypto/tls
  httpmux: alias of net/http.ServeMux
  zstd: optimized github.com/klauspost/compress backend
  sqlite: host-selected database/sql driver
  postgresql: upstream pgx stdlib, unmodified
  mysql: upstream github.com/go-sql-driver/mysql
tls_backends:
  linux: mbedTLS
  darwin: Secure Transport, with mbedTLS under -tags darwinstarttlswith13
  windows: Schannel
not_consumed:
  storage/s3: an S3 client for targets where aws-sdk-go-v2 does not build; no Popcorn Wave requirement depends on it
```
