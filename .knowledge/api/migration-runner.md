---
id: api:migration-runner
type: api
title: Migration Runner
---
The migration runner is the application-side surface that applies data:migration-source through whichever backend decision:migration-execution-split selects.

```yaml
opt_in: blank import of the migration engine package per decision:goose-migration-engine
surface:
  - Migrate(context.Context, ...MigrateOption) (Result, error)
  - Status(context.Context, ...MigrateOption) ([]VersionState, error)
  - WithDir(path string)
  - WithFS(fs.FS)
options:
  default_source: data:project-config migration.dir
  embedded_source: WithFS supplies an embed.FS of the same tree
result:
  - previous version
  - current version
  - applied versions in order
database:
  source: the pool owned by decision:config-driven-database
  rule: the runner never opens a second pool when the framework pool is available
framework_action:
  flag: --pw-print-dsn, a framework-owned action mirroring the api:subcommands built-in handling
  purpose: lets system:pw-cli read the effective DSN without reimplementing configuration precedence
  behavior: parse configuration, validate the configured driver and DSN, write the DSN to stdout, exit
  boundary: the value crosses a pipe to the parent process only and is never logged
startup:
  default: disabled
  enabled_by: explicit configuration only
  policy: policy:migration-safety
backends:
  in_process: goose Provider linked into a host Go build
  delegated: pw child process for a TinyGo build
  guarantee: identical Result, ordering, and error classification from both backends
replay: rule:multi-statement-sql-execution governs how a snapshot is executed
errors:
  - pending migration source is unreadable
  - recorded version is ahead of available sources
  - a migration failed and was rolled back where the dialect allows it
  - the delegated backend could not locate or run pw
```
