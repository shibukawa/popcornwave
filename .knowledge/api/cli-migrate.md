---
id: api:cli-migrate
type: api
title: pw migrate
---
pw migrate inspects and applies data:migration-source against the application's effective database using the goose engine linked into the pw binary, and applies the generated table set of requirement:dynamodb-migration when that store is configured.

```yaml
usage: pw migrate <action> [flags]
stores:
  selection: a store participates when its configuration section is enabled, so a project with both runs both from one command
  order: SQL first, then DynamoDB, because a failed relational schema is the more common blocker
  scoping: --store=rdb|dynamo restricts a run to one of them
  reason: requirement:dynamodb-store is a second kind of store, not a second database, so it belongs under the same verb rather than a second command
dynamo_actions:
  status: report the plan of flow:dynamodb-migration, which is the created, matching, and refused tables; this is the production-relevant action, because it is the drift check
  up: apply the plan, creating what is missing; a development and test action, since a production table comes from deployment tooling per requirement:dynamodb-migration
  print: write the registered table definitions to stdout, so deployment tooling declares what the code declares instead of restating it
  print_form: the key schema of every registered table under its resolved deployed name, with the decision:dynamodb-operational-configuration surface deliberately absent
  unsupported: version, up-by-one, up-to, down, down-to, create, validate, and snapshot have no meaning without versions, per decision:dynamodb-desired-state-migration
  unsupported_form: a clear error naming the action and the store, never a silent skip
  no_rollback: policy:dynamodb-migration-safety refuses every destructive change, so --yes has no DynamoDB use
  resolution: the decision:dynamodb-table-registry framework action supplies the resolved table set, the way --pw-print-dsn supplies the DSN
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
