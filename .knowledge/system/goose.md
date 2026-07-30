---
id: system:goose
type: system
title: goose Migration Engine
---
goose is the external versioned SQL migration engine adopted by Popcorn Wave for schema history, ordering, and apply state.

```yaml
module: github.com/pressly/goose/v3
pinned_baseline: v3.27.3
api:
  preferred: goose.NewProvider(dialect, *sql.DB, fs.FS, ...ProviderOption)
  rejected: package-level global registry and SetDialect/SetBaseFS mutation
capabilities_used:
  - lexical versioned SQL migration files
  - applied-version table state
  - up, up-by-one, up-to, down-to, status, and version actions
  - fs.FS source so embedded and on-disk trees share one code path
capabilities_excluded:
  - Go function migrations
  - goose CLI binary distribution
  - goose managed .env loading
dialects:
  first_class: sqlite3, postgres, and mysql, selected by rule:rdb-dsn-resolution
  present_but_unsupported: mssql, clickhouse, vertica, ydb, turso
  gate: decision:server-sql-support-tier
  pooling_note: the dialect is chosen from the resolved engine and the pool comes from decision:config-driven-database, so goose never opens its own connection
module_footprint:
  declared: the goose root go.mod requires driver, testing, and container-tooling modules including pgx, mysql, mssql, clickhouse, ydb, testify, and moby client
  linked_packages_measured:
    - github.com/mfridman/interpolate
    - go.uber.org/multierr
    - golang.org/x/sync/errgroup
    - github.com/sethvargo/go-retry
  effect: package-level pruning keeps every declared driver out of the built binary; only the four modules above enter go.mod
  containment: decision:goose-migration-engine
runtime_compatibility:
  host_go: supported
  tinygo: unsupported and unverified; see decision:migration-execution-split
references:
  - https://github.com/pressly/goose
  - https://pkg.go.dev/github.com/pressly/goose/v3
```
