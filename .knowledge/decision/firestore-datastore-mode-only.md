---
id: decision:firestore-datastore-mode-only
type: decision
title: The Firestore Store Is Datastore Mode Only, And Says So At Startup
---
Every Popcorn Web surface is named `firestore` after the product, and the one thing the name does not say — that the database must have been created in Datastore mode — is checked at startup rather than left to a runtime error.

```yaml
status: accepted
decided: 2026-08-05, following the firestorebind naming system:tinybind chose for the binding
naming:
  config_section: middleware.firestore
  package: database/firestore, per api:firestore-package
  stores: sessionstore/firestore, authstate/firestore, authstore/firestore, per rule:storage-package-layout
  backend_name: firestore, the value of session.backend and auth.backend
  upstream_pairing: the driver package is datastore and the binding is firestorebind, so this side follows the binding
  why_not_datastore: "datastore" is a common noun that names no product, and a configuration value a deployment types should name the service it is billed for
the_constraint_the_name_hides:
  fact: Firestore is created in one of two modes, and the mode is chosen at database creation and cannot be changed afterward
  datastore_mode: speaks the Datastore v1 API, which is what system:tinygodriver-firestore implements
  native_mode: a different API with a different client, listeners included, which nothing in this stack speaks
  the_trap: a deployment reading "firestore" in a configuration key and pointing it at an existing native-mode database has a correct-looking configuration and a store that cannot work
  why_the_error_is_bad_on_its_own: a native-mode database answers the Datastore endpoint with FAILED_PRECONDITION, which is the same status a missing composite index produces, so the two failures are indistinguishable to a reader
startup_check:
  what: one lookup of a reserved key in the configured database, per api:firestore-package
  passes: any answer, including the ordinary miss, because a miss proves the project, the database, the credential, the token, and the mode
  fails: FAILED_PRECONDITION, reported as a mode error naming the database and stating that Datastore mode is required and cannot be enabled on an existing database
  the_ambiguity_is_resolvable_here: the reserved key carries no filter and no order, so a composite index cannot be what the service is complaining about; that is what makes this probe able to say "mode" rather than "precondition"
  cost: one read per process start
what_this_check_also_buys:
  replaces: the verify_schema of data:dynamodb-runtime-config, which has nothing to verify here per decision:firestore-no-schema-application
  covers: a wrong project, a wrong named database, an unreadable credential file, a clock far enough out to mint a rejected token, and a service account with no Datastore permission
  does_not_cover: whether any composite index a later query needs exists, which nothing can check before the query runs
documentation_duty:
  what: the guide states the mode requirement where the backend is selected, and states that an existing native-mode database is not convertible
  why: the remedy is to create a second database, which is a decision a reader should meet before configuring the store rather than after
related:
  - requirement:firestore-store
  - api:firestore-package
  - system:tinygodriver-firestore
  - decision:firestore-no-schema-application
```
