---
id: rule:framework-owned-tables
type: rule
title: Framework Owned Tables
---
A table created by framework middleware carries the `popcornwave_` prefix and reaches the database through its own migration file, never through startup.

```yaml
naming:
  prefix: popcornwave_
  reason: an application reading its own schema can tell at a glance which tables it does not own
current_tables:
  popcornwave_session: sessionstore/sqlite login sessions
  popcornwave_authstate: authstate/sqlite single-use ceremony records
  popcornwave_auth_allowlist: plugin/auth pre-registration for policy:oidc-admission registered mode
  popcornwave_passkey_credential: the api:auth-credential-store default store for data:passkey-credential
  popcornwave_auth_bootstrap: the api:auth-credential-store default store for data:account-bootstrap-credential
conditional_verification:
  tables: popcornwave_passkey_credential and popcornwave_auth_bootstrap
  rule: a table is verified only when the selected mode reads it and the application installed no store of its own
  bootstrap: additionally only when registration or recovery actually issues a credential
  reason: a deployment asked for a table nothing will ever write to learns to ignore the startup refusal
  note: the migration still creates them, because one package publishes one file
migrations:
  location: the application migration directory, beside application migrations
  file_name: "{version}_init_popcornwave_{capability}.sql"
  identity: the name stem after the version, which the owning package publishes and never changes
  version: the next free version at the moment the file is written, exactly like an application migration
  no_reserved_range: a capability added later takes the next number, so nothing already applied is renumbered
  detection: a project already carries a capability when a file with that name stem exists, at any version
  source: the owning package publishes the exact file content, and a repository test fails when a copy drifts
  scaffolding: api:cli-init writes the files of the selected authentication mode; api:cli-add writes them into an existing project
non_relational_stores:
  applies_to: requirement:dynamodb-session-store and any later framework table on a store with no versioned migration
  naming: unchanged; the popcornwave_ prefix is the declared name, which rule:dynamodb-table-naming then maps to the deployed one
  creation_in_development: the owning package registers a table definition through decision:dynamodb-table-registry, and requirement:dynamodb-migration creates it
  creation_in_production: deployment tooling, from the definition api:cli-migrate prints, because such a table reads as part of the infrastructure
  no_file: there is no migration file to publish, identify, or renumber, because decision:dynamodb-desired-state-migration has no version sequence
  detection: a capability is present when its table definition is registered, rather than when a file with its name stem exists
  unchanged: startup still verifies and never creates while serving, which is the same rule reached by a different route
startup:
  action: verify only
  missing_table: refuse to serve and name the missing table, the migration that creates it, and the command that applies it
  shape_check: validate the column layout of an existing table
  forbidden: creating, altering, or dropping a table while serving
rules:
  - one migration file per owning package
  - the file name, not the version, identifies a framework migration
  - never renumber a migration a project may already have applied
  - a package never writes to a table another package owns
  - application migrations never modify a popcornwave_ table
  - records that expire without being consumed are swept, because expiry is logical
related:
  - api:session-store
  - api:cli-add
  - api:cli-session-schema
  - policy:migration-safety
```
