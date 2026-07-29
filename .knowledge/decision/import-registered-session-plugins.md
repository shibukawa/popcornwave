---
id: decision:import-registered-session-plugins
type: decision
title: Import-Registered Session Plugins
---
Session core does not import storage implementations; blank imports opt implementations and their configbind targets into the application binary.

```yaml
examples:
  rdb_session_plugin: import _ "popcornwave/plugin/session/rdb"
  server_database_engine: import _ "popcornwave/database/postgres"
  redis_session_plugin: import _ "popcornwave/plugin/session/redis"
boundaries:
  - plugin/session/rdb registers the RDB session backend but no database engine
  - database engine packages register into rule:rdb-dsn-resolution independently from the session plugin
  - pw links requirement:contrib-sqlite itself, because it is the scaffold default; a server engine is the application's own import
  - plugin/session/redis registers the Redis-compatible session backend and client integration
  - core session packages import neither backend plugin
effects:
  - only imported implementations contribute code and dependencies to binary size
  - only imported plugins contribute backend-specific configuration schema
  - missing selected backend or RDB driver is a startup error
contract: api:session-backend-plugin
```
