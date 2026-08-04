---
id: decision:auth-backend-selection
type: decision
title: One auth.backend Key Selects All Four Authentication Stores
---
A project names the storage behind the framework-owned authentication tables once, in `auth.backend`, and the four stores move together.

```yaml
status: accepted
decided: user 2026-08-01
question: how requirement:dynamodb-auth-backend is selected, given that plugin/auth opens four stores and today reads a database handle for all of them
answer:
  key: auth.backend, a new field of data:authentication-runtime-config
  values: rdb, the default and the current behavior, and dynamo
  scope: authstate, allowlist, credential, and bootstrap at once, plus data:revoked-token-record, which requirement:jwt-only-api-authentication added after this was decided
  scope_growth: the revocation list arrived as a fifth store and is implemented against rdb only; it is named here rather than left out, because a store nobody listed is one the dynamo backend will silently not cover
  linking: decision:import-registered-session-plugins; importing the backend package registers its factory under the name, so a project links the backend it runs and no other
  missing_import: a startup error naming the import line to add, per rule:storage-package-layout
one_key_for_four:
  reason: the four stores are one deployment's authentication state, and a project splitting them across two engines gains nothing and loses the shared unit of work of decision:dynamodb-auth-compensating-registration
  consequence: a mixed configuration is not expressible, which is the intent
  escape: an application that genuinely wants one store elsewhere installs it through api:auth-credential-store or api:auth-allowlist-store, which outrank the selected backend
precedence:
  installed_store_wins: SetCredentialStore, SetBootstrapStore, and the allowlist setter are read before the backend, unchanged from today
  effect: auth.backend chooses the framework default, and never overrides an application implementation
  reason: api:auth-credential-store already states that installing a store means the framework creates and verifies no table for that capability, and that stays true
rejected_import_alone:
  form: importing authstore/dynamo selects it, with no configuration key, as a session backend is not selected by import either
  why_not: session.backend already names its backend and the import only registers it, so import-alone would be the one selection in the framework with no name in configuration
  sharper: two linked backends with no key would have no defined winner
rejected_auto_detect:
  form: use DynamoDB when middleware.rdb is absent and middleware.dynamo is enabled
  why_not:
    - a project with both stores, which requirement:dynamodb-store calls the expected shape, has no answer
    - adding an rdb section for one unrelated table would silently move authentication storage
    - a startup summary could report the choice, and policy:startup-summary reporting a choice nothing configured is how a deployment learns to distrust it
validation:
  - an unknown auth.backend value fails startup naming the registered names
  - auth.backend dynamo without middleware.dynamo enabled fails startup, the way session.backend rdb without middleware.rdb does today
  - auth.backend rdb without middleware.rdb keeps the current error, which is now the backend's rather than the package's
implemented:
  built: 2026-08-05
  revocation_list: the fifth store arrived with requirement:jwt-only-api-authentication and is relational only; jwt_only reads it and the allowlist directly rather than through the backend, so a non-rdb auth.backend is refused for that mode when either is in play, rather than accepted as a key that silently does nothing
  registry: auth.RegisterBackend, with the relational backend registered by plugin/auth itself and the DynamoDB one by authstore/dynamo from init
  ceremony_store: a backend supplies authstate.RawStore per namespace rather than a typed store, because two of the three ceremony types are unexported
  sweep: a ceremony store that needs one implements Prune; a store whose expiry is decided on read implements nothing and the sweep skips it
  validation: an unlinked name is refused at configuration validation, naming what is linked
related:
  - requirement:dynamodb-auth-backend
  - data:authentication-runtime-config
  - api:session-backend-plugin
```
