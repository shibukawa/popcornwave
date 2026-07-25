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
    - resolve driver and DSN
    - open and validate the pool
    - install it in data:request-context-capsule
shutdown: api:application-lifecycle closes the pool
schema: requirement:database-migration uses the same effective configuration
application_code:
  forbidden:
    - pw.SetDatabase
    - explicit sqlite.Open for framework-managed requests
    - manual pool injection before Run
  allowed: api:request-context-accessors and generated SQL
```
