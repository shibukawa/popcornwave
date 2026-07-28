---
id: api:rdb-middleware
type: api
title: RDB Middleware
---
RDB middleware initializes and owns one shared *sql.DB and installs request SQL resources into data:request-context-capsule.

```yaml
startup:
  - read rdb fields from data:middleware-runtime-config
  - resolve the DSN scheme to a separately registered requirement:contrib-database driver
  - open *sql.DB and apply standard pool settings
  - ping within rdb.connect_timeout before accepting requests
  - construct standard http.Handler middleware
  - require no application pw.SetDatabase call under decision:config-driven-database
request:
  - install *sql.DB before downstream dispatch
  - make api:request-context-accessors available
  - do not begin, commit, or rollback a request transaction, per decision:explicit-transaction-boundary
  - active SQL executor defaults to *sql.DB, or to an existing data:transaction-scope transaction under api:test-run
  - executor resolution decorates that executor per api:instrumented-sql-executor when query diagnostics are enabled
constraints:
  - transaction boundaries come only from api:transaction-runner
  - streaming, connection hijacking, and early flush need no special transaction configuration
shutdown:
  - stop accepting requests and drain active handlers
  - close the owned *sql.DB through api:application-lifecycle
```
