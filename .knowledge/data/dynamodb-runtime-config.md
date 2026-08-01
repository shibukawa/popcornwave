---
id: data:dynamodb-runtime-config
type: data
title: DynamoDB Runtime Configuration
---
The middleware.dynamo section of data:middleware-runtime-config, registered by importing api:dynamo-package and independent of middleware.rdb.

```yaml
owner: api:dynamo-package
registration: an independent binding, per decision:independent-runtime-config-bindings
fields:
  enabled: bool, default false
  region: required unless the environment supplies it; empty with no environment value is a startup error
  endpoint: optional; empty selects the regional host, and a value selects a local or compatible server
  access_key_id: optional, expanded from ${NAME}; empty selects the driver's environment credentials
  secret_access_key: optional, expanded from ${NAME}; redacted everywhere
  session_token: optional, expanded from ${NAME}; redacted everywhere
  table_prefix: string, default empty, prepended to a declared name, per rule:dynamodb-table-naming
  table_names: an explicit declared-to-deployed map, default empty, for a deployed name no prefix produces
  timeout: duration, default 10s, the driver default restated so it is configurable
  max_idle_conns: non-negative int, default 4; guidance is to set it to the expected concurrency
  verify_schema: bool, default true; reads every registered table once at startup and refuses to serve on a mismatch
  auto_migrate: bool, default false, development only, governed by policy:dynamodb-migration-safety
no_billing_or_capacity_keys:
  removed: 2026-08-01
  reason: requirement:dynamodb-migration creates a table only in development and test, and a local emulator ignores billing mode and capacity entirely
  production: deployment tooling sets them, per decision:dynamodb-operational-configuration
  effect: a created table takes the driver default of on-demand billing, and no key here can say otherwise
validation:
  - enabled true requires a resolvable region, from this section or the environment
  - a static access_key_id requires a static secret_access_key, and neither alone is accepted
  - every resolved name satisfies the DynamoDB name rule, checked at startup over the whole decision:dynamodb-table-registry set rather than at the first request
  - a table_names entry naming an undeclared table is an error, since it would silently do nothing
  - the two keys compose into the one resolver function rule:dynamodb-table-naming installs, and neither is read anywhere else
  - timeout is positive and max_idle_conns is not negative
  - an unreachable endpoint is a startup error naming the endpoint with credentials redacted
  - auto_migrate set outside development is a configuration error, because a production table comes from deployment tooling
  - verify_schema false is accepted and warned about, since it removes the one check deployment tooling cannot make
secrets:
  mechanism: ${NAME} expansion in TOML string values, the same file-layer mechanism data:database-connection-set uses
  rule: an undefined name is a load error rather than an empty expansion
  redaction: the three credential keys are redacted after expansion, so an expanded secret never reaches policy:startup-summary or an error
scaffolded_development:
  endpoint: http://127.0.0.1:8000, the amazon/dynamodb-local address
  credentials: any non-empty pair, because the local server does not verify the signature
  region: a placeholder value the local server accepts
  rule: development-only values live in config.dev.toml and are never a deployment default
relation_to_rdb: none; middleware.rdb absent and middleware.dynamo enabled is a valid configuration, and so is the reverse
```
