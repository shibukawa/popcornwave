---
id: decision:firestore-no-schema-application
type: decision
title: There Is No Schema To Apply, So There Is No Migration Step
---
A kind comes into being on first write and no API reports one, so this store has no counterpart to requirement:dynamodb-migration; what replaces it is one startup probe and, for a query that needs one, a composite index descriptor the deployment applies.

```yaml
status: accepted
decided: 2026-08-05
premise: system:tinygodriver-firestore has no table admin at all, and the reason is the service rather than the driver
what_disappears:
  create: nothing to create; the first Put of a kind is the creation
  describe: nothing to observe, so the desired-versus-observed comparison of decision:dynamodb-desired-state-migration has no observed half
  verify_schema: the data:dynamodb-runtime-config key has nothing to read, and is absent from data:firestore-runtime-config rather than accepted and ignored
  auto_migrate: the same
  table_registry: decision:dynamodb-table-registry has no counterpart; a kind is intrinsic to the type, so nothing has to be enumerated for a migrator to find it
  key_schema_agreement: a key is a path of kind plus name, not a typed attribute pair, so the retyped-key hazard system:tinygodriver-dynamodb records has nothing to be hazardous about
what_this_removes_from_the_framework:
  - a migration entry point, a plan verb, and a CLI dispatch for this store
  - the development create step and the active-state poll after it
  - the startup refusal that names a missing table and the command that creates it
  - the test-run create and teardown of requirement:dynamodb-test-isolation, replaced by requirement:firestore-test-isolation
  reading: the DynamoDB store spends real machinery on making a table exist; here the machinery is not simplified, it is absent
what_replaces_the_startup_check:
  probe: one lookup of a reserved key, per decision:firestore-datastore-mode-only
  what_it_proves: the project, the named database, the credential, the token, the permission, and the mode
  what_it_cannot_prove: that a kind holds what this build expects, because nothing describes a kind
  honest_statement: a schema mismatch is not detectable here, and the guide says so rather than implying the probe covers it
  why_that_is_tolerable: the five framework kinds are written and read by one package each, so there is no second author for a shape to drift from; this is the same argument requirement:dynamodb-session-store gives for writing its item mapping by hand
composite_indexes:
  needed_when: a query combines an equality filter with an inequality or an order on another property
  needed_by_the_framework_stores: none of them
  why_none: every read the five stores perform is a key lookup, a batch lookup, or a single-property equality query, and single-property indexes are automatic
  the_one_to_watch: the credential listing of requirement:firestore-auth-stores, which is an equality on the account id and nothing else; adding an order to it would need an index, so it stays unordered
  when_an_application_needs_one: it declares a descriptor and its deployment applies it, which is what system:tinybind decided to emit for firestorebind; nothing here or there derives which index a query wants
  failure_mode: FAILED_PRECONDITION at run time on code that compiled, which the guide states beside any query surface this repository ships
publishing:
  what_the_cli_prints: for this store, the kinds the linked packages own and the TTL policies a deployment must configure, per decision:firestore-expiry-policy
  what_it_does_not_print: a table definition, since there is none
  why_print_anything: a deployment still has to be told which kinds exist and which timestamp property to point a TTL policy at, and that list is only knowable from the linked code
related:
  - requirement:firestore-store
  - requirement:dynamodb-migration
  - decision:firestore-datastore-mode-only
  - decision:firestore-expiry-policy
  - rule:framework-owned-tables
```
