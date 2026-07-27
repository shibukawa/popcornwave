---
id: decision:import-registered-session-plugins
type: decision
title: Import-Registered Session Plugins
---
Session core does not import storage implementations; blank imports opt implementations and their configbind targets into the application binary.

```yaml
examples:
  rdb_session_plugin: import _ "popcornwave/plugin/session/rdb"
  sqlite_database_driver: import _ "github.com/shibukawa/tinygodriver/database/sql/sqlite"
  redis_session_plugin: import _ "popcornwave/plugin/session/redis"
boundaries:
  - plugin/session/rdb registers the RDB session backend but no database/sql driver
  - database driver packages register independently from the session plugin
  - plugin/session/redis registers the Redis-compatible session backend and client integration
  - core session packages import neither backend plugin
effects:
  - only imported implementations contribute code and dependencies to binary size
  - only imported plugins contribute backend-specific configuration schema
  - missing selected backend or RDB driver is a startup error
contract: api:session-backend-plugin
```
