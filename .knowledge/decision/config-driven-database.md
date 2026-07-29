---
id: decision:config-driven-database
type: decision
title: Config-driven Database Lifecycle
---
The framework opens and owns the application database from validated runtime TOML configuration.

```yaml
status: accepted
configuration: data:middleware-runtime-config rdb fields
startup:
  owner: api:rdb-middleware
  steps:
    - resolve driver and DSN for every configured connection
    - open and validate each pool
    - install the data:database-connection-set in data:request-context-capsule
shutdown: api:application-lifecycle closes every pool
schema: requirement:database-migration uses the same effective configuration and the migration group of policy:connection-group-selection
topology: decision:grouped-database-connections extends this from one pool to named groups
application_code:
  forbidden:
    - pw.SetDatabase
    - explicit sqlite.Open for framework-managed requests
    - manual pool injection before Run
  allowed: api:request-context-accessors, api:database-selection, and generated SQL
```
