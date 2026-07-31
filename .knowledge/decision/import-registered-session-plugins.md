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
built_in_exception:
  backend: cookie
  reason: decision:cookie-session-storage stores records in the browser, so it adds no dependency to exclude
  effect: it is registered by pw and needs no import; every other backend needs one
boundaries:
  - plugin/session/rdb registers the RDB session backend but no database/sql driver
  - database driver packages register independently from the session plugin
  - plugin/session/redis registers the Redis-compatible session backend and client integration
  - core session packages import neither backend plugin
  - a host such as plugin/auth resolves the backend by name and imports no storage plugin either
effects:
  - only imported implementations contribute code and dependencies to binary size
  - a missing selected backend or RDB driver is a startup error
  - backend-specific keys are still declared by pw, so an unimported backend contributes no code but its keys remain visible
contract: api:session-backend-plugin
mechanism:
  problem: api:session-store is generic over the payload type, so a name-keyed registry cannot hold a typed factory
  solution: session.RawStore is the non-generic plugin contract over encoded payloads, and session.Typed adds the payload type back at the host
  registry: pw owns it, because pw already declares data:session-runtime-config and every plugin may import pw
missing_import:
  detection: startup, when the configured backend name resolves to no registered factory
  response: name the backend, list the registered ones, and print the import line that adds it
  reason: a linker error would name a symbol, while this names the one line an application is missing
```
