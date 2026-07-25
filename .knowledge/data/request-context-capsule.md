---
id: data:request-context-capsule
type: data
title: Request Context Capsule
---
Popcorn Wave stores stable request-scoped framework resources in one private value to bound context.Value lookup depth.

```yaml
visibility:
  type: private
  context_key: private
fields:
  database: optional *sql.DB
  transaction: optional *sql.Tx
  configuration_registry: data:runtime-config-registry
  root_span: optional contrib/otel/trace *Span
  logger: private api:logger backend and stable request attributes
  session: optional validated data:session-record safe view
  authentication: data:request-authentication
  csrf_token: optional masked request token
derived:
  sql_executor: transaction when present, otherwise database
lifecycle:
  - create once per request
  - framework middleware assembles fields before application handler dispatch
  - authentication middleware finalizes authenticated or unauthenticated state before application handler dispatch
  - policy:csrf-protection derives a safe masked token without exposing the stored secret
  - fields are stable after handler dispatch
  - api:transaction-runner creates a scoped child context when middleware.rdb.auto_transaction is false
  - never share a capsule across requests
constraints:
  - no general-purpose user value map
  - no exported field access
  - no user-visible setter or replacement API
```
