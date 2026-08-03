---
id: data:database-connection-set
type: data
title: Database Connection Set
---
The framework-owned set of named connection groups, each holding one or more *sql.DB pools opened from `[[middleware.rdb.connections]]`, which is the only form a database is configured in.

```yaml
owner: api:rdb-middleware
model:
  connection: one configured DSN, one *sql.DB, one driver
  group: one name, an ordered list of connections that address the same logical database
  set: every configured group plus the resolved default and write group pointers
connection_fields:
  group: required non-empty name, ASCII lower-case letters, digits, underscore, hyphen
  dsn: driver://dsn, resolved like the single-pool form
  readonly: bool, default false
  connect_timeout: duration, default 5s
  max_open_conns: non-negative int
  max_idle_conns: non-negative int
  conn_max_lifetime: non-negative duration
  conn_max_idle_time: non-negative duration
set_fields:
  default_group: group serving unpinned reads, optional when exactly one group exists
  write_group: group serving framework-owned writes, optional when exactly one writable group exists
identity:
  label: group name plus its one-based ordinal within the group, such as replica#2
  order: TOML element order within the group
  use: startup summary, data:query-record, and every configuration error message
removed_form:
  was: middleware.rdb.dsn and its sibling pool keys, expanded into one connection in group default
  now: removed; the section carries no DSN and no pool key of its own, so there is one way to configure a database
  reason: two forms meant every reader, every check, and every scaffold branched on which one a file used
  deployment_value: a ${NAME} reference in the element, since an array element has no environment variable or CLI option of its own
  stale_file: middleware.rdb.dsn is claimed by no binding, so an enabled rdb reads as having no pool, and the startup error names the element that replaces it
collapse:
  trigger: the set holds exactly one connection
  effect: every group name resolves to that connection, and a transaction on it accepts any group name
  reason: one development sqlite file and a reader-writer cluster must run the same application code
  applies_to: a single connections element and api:test-run
selection:
  round_robin: one atomic cursor per group, advanced once per group resolution
  memoization: the chosen connection is cached per context chain and group, so one request keeps one connection per group
  rationale: a request that reads twice from a replica group must not straddle two replicas with different lag
validation:
  - enabled requires at least one connections element
  - default_group and write_group must name a configured group
  - default_group is required once more than one group exists
  - a connections element naming no group takes the default group name
  - write_group must contain at least one connection with readonly false
  - a readonly connection cannot belong to a group used for framework-owned writes
  - all connections of one group must resolve the same driver scheme
  - per connection, connect_timeout must be positive and pool bounds must not be negative
  - per connection, max_idle_conns cannot exceed a positive max_open_conns
  - sqlite://:memory: requires max_open_conns 1 and rejects a group holding more than one connection, because each such DSN is a separate database
  - reject an unregistered DSN driver, a malformed DSN, or a failed startup ping on any connection
  - redact every DSN credential in config views, logs, errors, and the startup summary, per rule:dsn-redaction, which keeps the address and removes the credential
constraints:
  - the set is immutable after startup; no connection is added, removed, or ejected at runtime
  - decision:grouped-database-connections lists health checking, failover, replica-lag awareness, and weighted routing as non-goals
secret_input:
  problem: configbind gives an array element no CLI option and no environment variable, because its identity is its file position
  mechanism: ${NAME} expansion in TOML string values, owned by system:tinybind configbind
  scope: the file layer only; an environment or CLI value is taken verbatim
  undefined_name: load error, never an empty expansion, because the file layer outranks a default
  escape: $$ writes one literal $
  redaction: the dsn key is redacted after expansion per rule:dsn-redaction, so an expanded secret never reaches the startup summary or an error
```
