---
id: api:cli-migrate
type: api
title: pw migrate
---
pw migrate inspects and applies data:migration-source against the application's effective database using the goose engine linked into the pw binary.

```yaml
usage: pw migrate <action> [flags]
actions:
  status: list every version with applied state and pending count
  version: print the current applied version
  up: apply every pending migration
  up-by-one: apply the next pending migration
  up-to <version>: apply through a version
  down: roll back the newest applied migration
  down-to <version>: roll back to a version
  create <name>: write a new empty annotated SQL file
  validate: parse and order sources without touching the database
  snapshot: apply to a throwaway temporary database and write data:migration-snapshot to stdout
excluded_actions:
  reset: rolling every migration back is expressed as down-to 0
flags:
  --dir: override data:project-config migration.dir
  --dsn: bypass application configuration for operator use
  --yes: non-interactive confirmation required by policy:migration-safety for down and down-to
  --dry-run: report the planned actions without applying
engine:
  linkage: goose is statically linked into pw per decision:goose-migration-engine
  execution: pw connects and applies in its own process; no application binary is required
configuration_resolution:
  order:
    - --dsn when given
    - PW_MIGRATE_DSN from the environment
    - the application's --pw-print-dsn framework action, run through the host Go toolchain
  reason: configbind owns TOML, environment, and flag precedence, so the CLI never reimplements DSN resolution
  connection_group: --pw-print-dsn resolves the migration group of policy:connection-group-selection and prints its first connection DSN
  no_project_mode: an explicit --dir plus an explicit or environment DSN runs without popcornwave.toml, which is how a delegated child invokes it
  child_invocation: the same command serves decision:migration-execution-split delegation from a TinyGo binary
  snapshot_action: needs no application DSN because it creates and discards its own temporary database
behavior:
  - resolve the project root and data:project-config
  - resolve the effective database through decision:config-driven-database
  - open and close its own connection for the command
  - stop on the first failure and return nonzero
  - print applied versions and elapsed time per migration
  - redact credentials from all output and errors
constraints:
  - down and down-to follow policy:migration-safety confirmation rules
  - a rollback plan lists migrations newest first, matching the order they are reversed in
  - create writes only into the configured migration directory
  - the command never runs api:cli-generate or a build
```
