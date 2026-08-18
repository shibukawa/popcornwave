---
id: rule:framework-owned-tables
type: rule
title: Framework Owned Tables
---
A table created by framework middleware carries the `popcornweb_` prefix and reaches the database through its own migration file, never through startup.

```yaml
naming:
  prefix: popcornweb_
  reason: an application reading its own schema can tell at a glance which tables it does not own
current_tables:
  popcornweb_session: sessionstore/sqlite login sessions
  popcornweb_authstate: authstate/sqlite single-use ceremony records
  popcornweb_auth_allowlist: plugin/auth pre-registration for policy:oidc-admission registered mode
  popcornweb_passkey_credential: the api:auth-credential-store default store for data:passkey-credential
  popcornweb_auth_bootstrap: the api:auth-credential-store default store for data:account-bootstrap-credential
conditional_verification:
  tables: popcornweb_passkey_credential, popcornweb_auth_bootstrap, and popcornweb_auth_allowlist
  rule: a table is verified only when the selected mode reads it and the application installed no store of its own
  allowlist: read only when auth.oidc.admission is registered, and skipped when api:auth-allowlist-store carries an installed store
  bootstrap: additionally only when registration or recovery actually issues a credential
  reason: a deployment asked for a table nothing will ever write to learns to ignore the startup refusal
  note: the migration still creates them, because one package publishes one file
migrations:
  location: the application migration directory, beside application migrations
  file_name: "{version}_init_popcornweb_{capability}.sql"
  identity: the name stem after the version, which the owning package publishes and never changes
  version: the next free version at the moment the file is written, exactly like an application migration
  no_reserved_range: a capability added later takes the next number, so nothing already applied is renumbered
  detection: a project already carries a capability when a file with that name stem exists, at any version
  source: the owning package publishes the exact file content, and a repository test fails when a copy drifts
  scaffolding: api:cli-init writes the files of the selected authentication mode; api:cli-add writes them into an existing project
non_relational_stores:
  applies_to: requirement:dynamodb-session-store, requirement:dynamodb-auth-stores, requirement:contrib-auth-state-dynamo, and any later framework table on a store with no versioned migration
  naming: unchanged; the popcornweb_ prefix is the declared name, which rule:dynamodb-table-naming then maps to the deployed one
  creation_in_development: the owning package registers a table definition through decision:dynamodb-table-registry, and requirement:dynamodb-migration creates it
  creation_in_production: deployment tooling, from the definition api:cli-migrate prints, because such a table reads as part of the infrastructure
  no_file: there is no migration file to publish, identify, or renumber, because decision:dynamodb-desired-state-migration has no version sequence
  detection: a capability is present when its table definition is registered, rather than when a file with its name stem exists
  unchanged: startup still verifies and never creates while serving, which is the same rule reached by a different route
schemaless_stores:
  applies_to: requirement:firestore-session-store, requirement:firestore-auth-stores, requirement:contrib-auth-state-firestore, and any later framework kind on a store that reports no schema
  naming: unchanged; the popcornweb_ prefix names the kind, and no resolver maps it, per decision:firestore-namespace-isolation
  creation: none anywhere; a kind exists on first write, per decision:firestore-no-schema-application
  detection: a capability is present when its package is linked, which is the only signal left once nothing is registered and no file is published
  what_startup_does_instead: the reachability and mode probe of decision:firestore-datastore-mode-only, which proves the database and not the shape
  why_the_verification_half_disappears: nothing reports a kind, so there is no observed state to compare a desired one against; this rule's verify-only stance is not weakened here, it has no object
  isolation_from_deployment_tooling: a deployment still configures the expiry policies decision:firestore-expiry-policy names, which is the one thing it must be told about these kinds
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
  - application migrations never modify a popcornweb_ table
  - records that expire without being consumed are swept, because expiry is logical
related:
  - api:session-store
  - api:cli-add
  - api:cli-session-schema
  - policy:migration-safety
```
