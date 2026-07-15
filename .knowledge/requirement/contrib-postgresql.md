---
id: requirement:contrib-postgresql
type: requirement
title: TinyGo PostgreSQL Driver
---
contrib/database/postgres implements PostgreSQL protocol 3 clients for common database/sql operations.

```yaml
package: contrib/database/postgres
required:
  - TCP connection and TLS negotiation
  - startup parameters
  - cleartext password only when TLS and explicitly enabled
  - MD5 authentication for legacy interoperability
  - SCRAM-SHA-256 authentication
  - simple query
  - extended query with typed parameters
  - prepared statements
  - transactions
  - null and core scalar decoding
  - ErrorResponse code extraction
  - cancellation request on context cancellation
deferred:
  - COPY
  - replication
  - pipeline mode
  - GSSAPI and OAUTHBEARER
  - binary codecs beyond core scalar types
protocol: https://www.postgresql.org/docs/current/protocol.html
```
