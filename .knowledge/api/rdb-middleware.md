---
id: api:rdb-middleware
type: api
title: RDB Middleware
---
RDB middleware initializes and owns the data:database-connection-set and installs request SQL resources into data:request-context-capsule.

```yaml
startup:
  - read rdb fields from data:middleware-runtime-config
  - expand the legacy single-DSN form into one connection in group default
  - resolve each connection DSN scheme to a separately registered requirement:contrib-database driver
  - open one *sql.DB per connection and apply its own pool settings
  - ping each connection within its connect_timeout before accepting requests
  - resolve default_group and write_group per policy:connection-group-selection
  - report every connection in the policy:startup-summary output with its label and redacted DSN
  - construct standard http.Handler middleware
  - require no application pw.SetDatabase call under decision:config-driven-database
request:
  - install the connection set before downstream dispatch
  - make api:request-context-accessors and api:database-selection available
  - do not begin, commit, or rollback a request transaction, per decision:explicit-transaction-boundary
  - active SQL executor comes from the api:database-selection resolution order
  - executor resolution decorates that executor per api:instrumented-sql-executor when query diagnostics are enabled
readiness:
  - the readiness endpoint pings every configured connection
  - a replica that cannot answer makes the instance unready, because the default group is what the application reads from
constraints:
  - transaction boundaries come only from api:transaction-runner
  - streaming, connection hijacking, and early flush need no special transaction configuration
  - the set is fixed at startup, per decision:grouped-database-connections
shutdown:
  - stop accepting requests and drain active handlers
  - close every owned *sql.DB through api:application-lifecycle
```
