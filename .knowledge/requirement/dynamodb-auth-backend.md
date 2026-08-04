---
id: requirement:dynamodb-auth-backend
type: requirement
title: DynamoDB Authentication Backend
---
plugin/auth runs with no relational database, so a project that already has requirement:dynamodb-store and requirement:dynamodb-session-store can authenticate without adding one.

```yaml
motivation:
  - requirement:dynamodb-session-store made the session backend RDB-free, and plugin/auth still refuses to start without middleware.rdb, so that store is unreachable through the framework's own authentication
  - requirement:dynamodb-store allows a project with no rdb section, and such a project cannot log anyone in today
  - the four stores below are the only reason plugin/auth touches a database at all; nothing else in the package is relational
current_gate:
  where: plugin/auth setup, before any store is opened
  behavior: a project with no pw.DB fails with "auth requires middleware.rdb.enabled = true"
  second_gate: the same setup resolves pw.SelectSessionDB and passes the handle as a session resource, which requirement:dynamodb-session-store ignores, so the handle exists only to satisfy the check
  removal: the gate becomes a per-backend requirement, asserted by the selected backend rather than by the package
four_stores:
  authstate:
    contract: requirement:contrib-auth-state
    today: an authstate SQL store constructed directly in setup
    dynamo: requirement:contrib-auth-state-dynamo, specified and unimplemented
  allowlist:
    contract: none; raw SQL against popcornwave_auth_allowlist
    used_by: policy:oidc-admission registered mode
    needed_first: api:auth-allowlist-store, because this is the only one of the four with no seam to install an implementation behind
    dynamo: requirement:dynamodb-auth-stores
  credential:
    contract: api:auth-credential-store CredentialStore
    dynamo: requirement:dynamodb-auth-stores
  bootstrap:
    contract: api:auth-credential-store BootstrapStore
    dynamo: requirement:dynamodb-auth-stores
selection: decision:auth-backend-selection
atomicity: decision:dynamodb-auth-compensating-registration
tables:
  declared_names: unchanged, per rule:framework-owned-tables
  deployed_names: resolved by rule:dynamodb-table-naming, so requirement:dynamodb-test-isolation reaches them unchanged
  creation: registered through decision:dynamodb-table-registry when the backend package is imported, so requirement:dynamodb-migration creates them with every other table
  no_migration_file: per decision:dynamodb-desired-state-migration; the goose file plugin/auth publishes stays the SQL half
  startup: verify only, never create, which rule:framework-owned-tables already states for a non-relational store
scaffolding:
  api:cli-init: writes no auth migration file for a project selecting this backend, because there is none to write
  api:cli-add: the same
  verification: startup names the missing table and the pw migrate run that creates it, not a migration file
acceptance:
  - a project with middleware.rdb absent, middleware.dynamo enabled, session.backend dynamo, and auth.backend dynamo completes an OIDC login and a passkey login
  - the same project completes a passkey enrollment for an already active account
  - oidc.admission registered admits a pre-registered identity and denies an unregistered one, and a backend failure is an error rather than a denial, per policy:oidc-admission
  - every table this backend needs is created by the same pw migrate run that creates application tables
  - startup refuses to serve when one of them is absent, naming the table and the command
  - a project keeping middleware.rdb and auth.backend rdb is unchanged in every observable way
  - an application that installs its own CredentialStore or BootstrapStore keeps that store whichever backend is selected
  - no request any of these stores issues produces a query-diagnostics record, per policy:query-log-safety
non_goals:
  - a portable interface over the relational and DynamoDB auth stores beyond the seams that already exist
  - moving data:user-account, data:external-identity, or the AccountResolver into the framework; account storage stays application-owned
  - sharing one table across the four stores, which would be the single-table design system:tinybind declines
  - migrating existing relational auth data into DynamoDB
```
