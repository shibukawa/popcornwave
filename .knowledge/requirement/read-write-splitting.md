---
id: requirement:read-write-splitting
type: requirement
title: Read-Write Split Connections
---
An application deployed against a reader-writer cluster reads from replicas by default and writes to the writer, by configuring connections rather than by wiring pools in code.

```yaml
audience: actor:application-developer
capabilities:
  grouped_connections: name several pools and address them as one group
  default_read: unpinned SQL uses default_group, balanced round robin across its connections
  explicit_selection: api:database-selection pins a group for one call or one handler
  scoped_transaction: api:transaction-runner takes OnGroup and keeps the whole callback on that group
  readonly_marking: a readonly connection rejects a write statement instead of forwarding it
configuration: data:database-connection-set
decision: decision:grouped-database-connections
selection: policy:connection-group-selection
lifecycle: api:rdb-middleware
acceptance:
  - an existing middleware.rdb.dsn configuration keeps working unchanged, as one connection in group default
  - declaring both middleware.rdb.dsn and a connections element fails startup with both key paths named
  - two connections sharing one group alternate across successive requests
  - one request that reads twice from a group uses one connection for both statements
  - unpinned SQL resolves default_group; SelectDB with a group name resolves that group
  - unpinned SQL inside a writer transaction runs on the writer, not on default_group
  - Transaction with OnGroup commits on that group, and a nested call on the same group opens a savepoint
  - a nested Transaction naming a different group fails and leaves the outer transaction usable
  - SelectDB to a readonly group inside a transaction reads outside it; to a writable group it fails
  - a write against a readonly connection fails as a framework error once the sqlbind resolver contract carries a statement access mode, per decision:grouped-database-connections
  - a readonly connection configured as write_group, session group, or migration group fails startup
  - migration, seeding, and the session store write to their resolved group, never to a replica
  - api:test-run maps every group name onto its single test pool, so a SelectDB call needs no test-only branch
  - a single-connection deployment answers every group name too, so one sqlite file runs the same code as a cluster
  - a partly opened set is closed rather than served when one connection fails to open
  - the startup summary lists every connection with its group, readonly flag, pool bounds, and redacted DSN
  - a failed ping on any connection stops startup and names the connection label
non_goals:
  - health checking, ejection, or failover among replicas
  - replica-lag awareness or read-your-writes routing
  - weighted or latency-aware balancing
  - transactions spanning two groups
  - sharding or per-tenant connection routing
```
