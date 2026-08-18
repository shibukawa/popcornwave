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
gate_that_was_lifted:
  where: plugin/auth setup, before any store was opened
  behavior: a project with no pw.DB failed with "auth requires middleware.rdb.enabled = true"
  second_gate: the same setup resolved pw.SelectSessionDB and passed the handle as a session resource, which requirement:dynamodb-session-store ignores, so the handle existed only to satisfy the check
  reached_the_cli:
    - api:cli-init refused an authentication mode backed only by DynamoDB, because the runtime would have refused to start
    - decision:interactive-project-bootstrap asked the database-engine question even when DynamoDB was the store answer, for the same reason
  removal: the gate is now a per-backend requirement, asserted by the selected backend rather than by the package, so scaffolding and startup changed together
four_stores:
  authstate:
    contract: requirement:contrib-auth-state
    was: an authstate SQL store constructed directly in setup
    dynamo: requirement:contrib-auth-state-dynamo
  allowlist:
    contract: api:auth-allowlist-store, added here because this was the only one of the four with no seam to install an implementation behind
    was: raw SQL against popcornweb_auth_allowlist
    used_by: policy:oidc-admission registered mode
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
  api:cli-init: offers an authentication mode on a DynamoDB-only project, and writes no auth migration file for it, because there is none to write
  wizard: decision:interactive-project-bootstrap stops asking for a database engine when DynamoDB is the store answer and a login is wanted
  api:cli-add: the same
  verification: startup names the missing table and the pw migrate run that creates it, not a migration file
acceptance:
  - a project with middleware.rdb absent, middleware.dynamo enabled, session.backend dynamo, and auth.backend dynamo completes an OIDC login and a passkey login
  - the same project completes a passkey enrollment for an already active account
  - oidc.admission registered admits a pre-registered identity and denies an unregistered one, and a backend failure is an error rather than a denial, per policy:oidc-admission
  - every table this backend needs is created by the same pw migrate run that creates application tables
  - startup refuses to serve when one of them is absent, naming the table and the command
  - pw init offers an authentication mode on a project that selected DynamoDB and no relational database, and the project it writes serves
  - a project keeping middleware.rdb and auth.backend rdb is unchanged in every observable way
  - an application that installs its own CredentialStore or BootstrapStore keeps that store whichever backend is selected
  - no request any of these stores issues produces a query-diagnostics record, per policy:query-log-safety
implemented:
  built: 2026-08-05
  verified: a scaffolded passkey_only project with middleware.rdb absent, middleware.dynamo enabled, session.backend and auth.backend dynamo starts, creates all five framework tables, and completes a passkey ceremony against DynamoDB Local
  fixed_along_the_way:
    - middleware.dynamo carried no bound defaults, so every project enabling it failed startup on a zero timeout; the generator test covered only two of the three committed bindings, which is why it drifted unnoticed
    - the session setup resolved the database before asking the backend, so requirement:dynamodb-session-store could not start either
    - sessionstore/dynamo read its client from the setup context, which never carries one; api:dynamo-package now exposes EnsureClient
    - pw init offered no dynamo session backend, so a DynamoDB-only project would have scaffolded a relational session
non_goals:
  - a portable interface over the relational and DynamoDB auth stores beyond the seams that already exist
  - moving data:user-account, data:external-identity, or the AccountResolver into the framework; account storage stays application-owned
  - sharing one table across the four stores, which would be the single-table design system:tinybind declines
  - migrating existing relational auth data into DynamoDB
```
