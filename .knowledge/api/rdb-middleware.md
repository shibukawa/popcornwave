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
  common:
    - install *sql.DB before downstream dispatch
    - make api:request-context-accessors available
  auto_transaction:
    - begin *sql.Tx before downstream dispatch
    - install the transaction as the active SQL executor
    - buffer non-streaming response until transaction outcome is known
    - commit on a completed response with status below 400
    - publish the buffered response only after commit succeeds
    - rollback on status 400 or greater, panic, cancellation, write failure, or commit failure
  manual_transaction:
    - do not begin, commit, or rollback a request transaction
    - active SQL executor defaults to *sql.DB
constraints:
  - rdb.auto_transaction selects auto_transaction when true and manual_transaction when false
  - automatic transactions cover every HTTP method, including GET and HEAD
  - streaming, connection hijacking, and early flush require auto_transaction false
  - rollback errors are observed and logged without replacing the primary failure
  - commit failure becomes a safe HTTP 500 because no response was published
shutdown:
  - stop accepting requests and drain active handlers
  - close the owned *sql.DB through api:application-lifecycle
```
