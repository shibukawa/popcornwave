---
id: requirement:contrib-postgresql
type: requirement
title: TinyGo PostgreSQL Driver
---
contrib/database/postgres is a deferred non-first-class investigation and is unsupported by Petitweb releases under decision:server-sql-support-tier.

```yaml
package: contrib/database/postgres
support_tier: non_first_class
compatibility_label: unsupported
status: deferred
blocker: decision:server-sql-support-tier
promotion_requirements:
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
product_boundaries:
  - no scaffold or default dependency
  - no compatibility guarantee
  - no release acceptance gate
deferred:
  - COPY
  - replication
  - pipeline mode
  - GSSAPI and OAUTHBEARER
  - binary codecs beyond core scalar types
protocol: https://www.postgresql.org/docs/current/protocol.html
```
