---
id: decision:grouped-database-connections
type: decision
title: Grouped Database Connections
---
middleware.rdb owns an array of named connection groups instead of one pool, so a reader-writer topology such as Aurora is configuration rather than application wiring.

```yaml
status: accepted
enabled_by: configbind array-of-tables binding in system:tinybind
extends: decision:config-driven-database
state: data:database-connection-set
model:
  unit: group, not role
  reason:
    - a role enum fixes the vocabulary; a free name lets an application add reporting or analytics groups
    - readonly stays a separate boolean so a group name never implies write capability
    - several connections share one group name, which is what makes round robin expressible
routing:
  unpinned_read: default_group, expected to be the replica group
  explicit: api:database-selection pins a group for one context
  framework_write: write_group, or a per-purpose override, per policy:connection-group-selection
  within_group: round robin, memoized per context chain
default_group_risk:
  observation: pointing default_group at a readonly group makes an unpinned write fail
  accepted: yes, because that failure is the point; a silent write to a replica is worse than a rejected one
  mitigation: the read-only executor option turns it into a framework error instead of a driver error
readonly_enforcement:
  owner: system:tinybind
  intended: the framework marks a readonly connection with the sqlbind read-only executor option, and sqlbind rejects a write statement
  framework_scope: pass the option; classify no SQL
  blocked_by:
    finding: sqlbind emits the read-only-aware write resolver only when no framework executor resolver is configured
    cause: the framework resolver contract is func(context.Context) (SQLExecutor, error), which carries no statement access mode
    effect: popcornwave configures that resolver, so generated writes never reach the check and the option has no effect
    needs: a resolver contract that carries the access mode, or a second resolver symbol for write statements
  in_effect_today:
    - a depth 0 transaction on a readonly connection begins with sql.TxOptions read-only
    - api:database-selection rejects selecting a writable group inside a transaction on another group
    - the database itself rejects the remaining writes
  seam: one framework function marks the resolved executor, so wiring the option is a one-place change once the contract exists
transaction_scope:
  rule: one transaction never spans two groups
  reason: two pools are two connections, and the framework adds no two-phase commit
  surface: api:transaction-runner rejects a nested call naming a different group
compatibility:
  legacy: middleware.rdb.dsn keeps working as one connection in group default
  removed: nothing
  mixing: declaring both forms is a startup error rather than a merge
non_goals:
  - health checking or automatic ejection of a failing replica
  - failover or promotion
  - replica-lag measurement or read-your-writes routing
  - weighted or latency-aware balancing
  - cross-group distributed transaction
rejected_alternatives:
  - fixed writer and reader keys, which cannot express more than two pools
  - a driver-level or proxy-level splitter, which hides the choice from application code and from data:query-record
  - per-statement round robin, which lets one request observe two replica lags
```
