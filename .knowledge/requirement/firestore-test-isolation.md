---
id: requirement:firestore-test-isolation
type: requirement
title: Isolated Firestore Tests
---
Firestore tests stay independent by giving each test server its own namespace, because the store has no rollback to isolate with and no table set to create and drop.

```yaml
problem: requirement:parallel-database-tests isolates SQL tests inside one rolled-back transaction, and a Datastore transaction is a unit of work rather than a container a test can wrap
mechanism:
  namespace: api:test-run assigns a unique data:firestore-runtime-config namespace per test server, per decision:firestore-namespace-isolation
  create: nothing; a kind exists on first write, so there is no pre-test apply step and no active-state poll
  parallelism: two test servers never address one key, so t.Parallel needs no further coordination
  second_client: a test installs a second context rather than a second signature, which is what makes two servers in one process independent
  one_value: the namespace covers every framework kind at once, so a test cannot half-isolate itself by forgetting one name
cleanup:
  problem: there is no operation that deletes a namespace, unlike the DeleteTable requirement:dynamodb-test-isolation ends a run with
  emulator: the emulator is started without persistence, so a run leaves nothing behind and needs no teardown at all
  real_project: QueryKeysPage per framework kind inside the run's namespace, then RemoveKeys over what it returns
  two_calls_and_no_chunker: firestorebind v0.3.6 added RemoveKeys for this shape, and it sizes each deletion with the driver's own MutationSize, so nothing here counts bytes or stamps a namespace
  why_that_is_acceptable: it runs only against a real project, which is already the slow path, and the kinds are enumerable because api:firestore-package Kinds reports what is linked
  failure: a teardown that fails leaves entities in a namespace nothing else reads, which costs storage and never affects another run
endpoint:
  development: the gcloud Datastore emulator, started on a pinned host and port because the default port is reassigned on restart
  container: the google/cloud-sdk image, since the emulator is a Java process inside the SDK rather than a standalone binary
  weight: heavier than amazon/dynamodb-local, which api:cli-dev has to account for in startup time and in what it asks a developer to have installed
  wrong_emulator: the Firestore emulator serves native mode and cannot answer this API, per decision:firestore-datastore-mode-only; a suite pointed at it fails on every call
  credentials: none configured; with the emulator endpoint set the driver sends no Authorization header
  real_project: supported and slow, and it is the only place the credential path runs at all
what_the_emulator_does_not_cover:
  auth: the emulator ignores the Authorization header entirely, so the token path, the audience, and the clock have no offline coverage; a sharper gap than DynamoDB Local's, where a signature must at least be present and well formed
  mode: an emulator in Datastore mode cannot reproduce the native-mode failure decision:firestore-datastore-mode-only exists to name
  indexes: the emulator's index behaviour is a reimplementation, so a query that needs a composite index may pass locally and fail in production
  contention: ABORTED cannot be provoked reliably, so the transaction retry paths of decision:firestore-conditional-writes are asserted against a stub rather than against the emulator
  consequence: one manual run against a real project per release covers the credential path and the index behaviour, which is what the upstream driver already concluded for itself
test_author_responsibility:
  - avoid asserting on a kind-wide count, which sees only this namespace but also sees every entity a shared-namespace test left
  - keep the entity key distinct per test where a suite deliberately shares one namespace
acceptance:
  - two test servers running in parallel write into disjoint namespaces and produce stable results
  - a suite against the emulator needs no Google Cloud project and no credential
  - a suite against a real project removes its own namespace's entities at teardown
  - a failing test leaves nothing another run can observe
  - a store call reaches the run's namespace without the test naming it, because the namespace is a client option rather than a call argument
seeding: none yet; requirement:dynamodb-test-data has no Firestore counterpart, and a fixture format for entities is not part of the first cut of requirement:firestore-store
non_goals:
  - transactional rollback around a whole test, which the store does not offer
  - sharing one namespace across parallel tests and cleaning entities between them
  - a namespace per test rather than per test server, which would multiply the teardown without buying isolation the server boundary does not already give
related:
  - requirement:parallel-database-tests
  - requirement:dynamodb-test-isolation
  - api:test-run
```
