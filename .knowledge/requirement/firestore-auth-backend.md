---
id: requirement:firestore-auth-backend
type: requirement
title: Firestore Authentication Backend
---
plugin/auth runs on Firestore in Datastore mode, so a project that already has requirement:firestore-store and requirement:firestore-session-store can authenticate without adding a relational database.

```yaml
motivation:
  - requirement:firestore-session-store makes the session backend relational-free on Google Cloud, and an authentication path that still needed middleware.rdb would make that store unreachable through the framework's own login
  - requirement:dynamodb-auth-backend already lifted the package-level rdb gate and turned it into a per-backend assertion, so this backend registers itself rather than removing a gate a second time
  - the five stores below are the only reason plugin/auth touches a database at all
what_is_already_done:
  gate: removed by requirement:dynamodb-auth-backend; plugin/auth no longer requires pw.DB and no longer resolves a session database to satisfy a check
  registry: auth.RegisterBackend exists, and a backend package registers its factory from init
  ceremony_contract: authstate.RawStore and authstate.Typed exist, added for the DynamoDB adapter and reused unchanged
  seam: auth.FirstEnrollmentStore exists, added for decision:dynamodb-auth-compensating-registration and used here to commit rather than to order
  ensure_client: the lesson that a store cannot read its client from the setup context is already learned; api:firestore-package exposes EnsureClient from the start
  effect: this requirement adds a backend and changes no shared machinery, which the DynamoDB one could not say
five_stores:
  authstate:
    contract: requirement:contrib-auth-state
    firestore: requirement:contrib-auth-state-firestore
  allowlist:
    contract: api:auth-allowlist-store
    firestore: requirement:firestore-auth-stores
  credential:
    contract: api:auth-credential-store CredentialStore
    firestore: requirement:firestore-auth-stores
  bootstrap:
    contract: api:auth-credential-store BootstrapStore
    firestore: requirement:firestore-auth-stores
  revocation:
    record: data:revoked-token-record
    firestore: none
    why: requirement:jwt-only-api-authentication implemented it against rdb only and reads it and the allowlist directly rather than through the backend
    behavior: auth.backend firestore is refused for jwt_only when either is in play, which is what decision:auth-backend-selection already does for dynamo, rather than accepted as a key that silently does nothing
selection: decision:auth-backend-selection, with firestore as a third registered value
atomicity: decision:firestore-conditional-writes, where the first-enrollment pair is one commit rather than the compensating sequence decision:dynamodb-auth-compensating-registration needed
kinds:
  names: unchanged, per rule:framework-owned-tables
  creation: none; a kind exists on first write, per decision:firestore-no-schema-application
  startup: the mode and reachability probe of decision:firestore-datastore-mode-only, which is the whole of what startup can check
  no_migration_file: there is none to publish, identify, or renumber
  no_registry_entry: nothing enumerates a kind for a migrator, unlike decision:dynamodb-table-registry
scaffolding:
  api:cli-init: offers an authentication mode on a Firestore-only project, and writes no auth migration file for it
  wizard: decision:interactive-project-bootstrap stops asking for a database engine when Firestore is the store answer and a login is wanted, the same branch DynamoDB already added
  api:cli-add: the same
  session_backend: pw init offers the firestore session backend, so a Firestore-only project does not scaffold a relational session; this is the exact omission requirement:dynamodb-auth-backend found on implementation
  verification: startup names a credential, project, database, or mode problem; it never names a missing kind, because a missing kind is not a state that exists
  development_server:
    what: the scaffolded configuration points at 127.0.0.1:8081 and names the gcloud command that serves it
    not_started_for_you: unlike amazon/dynamodb-local, the Datastore emulator is a Java process inside the Cloud SDK rather than a standalone server, so no Devbox package is added and pw dev starts nothing
    why_that_is_stated_rather_than_hidden: a scaffold that listed a package it could not run would fail at devbox shell instead of at the line that names the command
acceptance:
  - a project with middleware.rdb absent, middleware.firestore enabled, session.backend firestore, and auth.backend firestore completes an OIDC login and a passkey login
  - the same project completes a passkey enrollment for an already active account
  - the same project completes a passkey-only first enrollment, and an interruption before the commit leaves neither a spent bootstrap record nor a credential
  - oidc.admission registered admits a pre-registered identity and denies an unregistered one, and a backend failure is an error rather than a denial, per policy:oidc-admission
  - auth.backend firestore without middleware.firestore enabled fails startup
  - auth.backend firestore with an authentication mode that reads data:revoked-token-record fails startup naming the mode and the store, rather than starting and failing at the first revocation check
  - pw init offers an authentication mode on a project that selected Firestore and no relational database, and the project it writes serves against the emulator
  - a project keeping middleware.rdb and auth.backend rdb is unchanged in every observable way
  - a project on auth.backend dynamo is unchanged in every observable way
  - an application that installs its own CredentialStore or BootstrapStore keeps that store whichever backend is selected
  - no request any of these stores issues produces a query-diagnostics record, per policy:query-log-safety
implemented:
  built: 2026-08-05
  registry: auth.RegisterBackend under auth.BackendFirestore, from authstore/firestore init
  scaffolding: pw init offers Firestore as a store answer and as a session backend, --firestore selects it, and pw add firestore installs it into an existing project
  refused_pairing: a login with both non-relational stores and no relational database, since auth.backend names one store for all four kinds and there would be no defined winner
  verified: a scaffolded passkey_only Firestore-only project compiles, starts, and answers its health endpoint with middleware.rdb absent
non_goals:
  - a portable interface over the relational, DynamoDB, and Firestore auth stores beyond the seams that already exist
  - moving data:user-account, data:external-identity, or the AccountResolver into the framework
  - sharing one kind across the stores, which would be the single-table design system:tinybind declines
  - migrating existing relational or DynamoDB auth data into Firestore
  - a Firestore revocation list, until requirement:jwt-only-api-authentication has a backend seam to install one behind
```
