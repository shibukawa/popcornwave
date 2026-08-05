---
id: decision:firestore-namespace-isolation
type: decision
title: A Namespace Replaces The Table Prefix
---
Where DynamoDB isolates a deployment or a test run by resolving every declared table name onto a deployed one, Firestore carries a namespace on every key, so the isolation dimension is configuration on the client and no store builds a name.

```yaml
status: accepted
decided: 2026-08-05
what_rule_dynamodb_table_naming_solved:
  problem: a declared table name and a deployed one differ, and every call site would otherwise build the deployed string
  answer: a resolver function installed once, composed from a prefix and an explicit map
why_it_has_no_counterpart:
  a_kind_is_intrinsic: an entity of kind popcornwave_session is one wherever it is stored, which is why system:tinybind puts the kind on the type rather than in configuration; a deployment does not rename it
  nothing_to_resolve: firestorebind offers no table resolver for exactly this reason, unlike the dynamobind option rule:dynamodb-table-naming needs
  so_the_kind_stays_literal: one constant per store, and no mapping layer between the source and the wire
what_the_namespace_does_instead:
  scope: every key in the database, across every kind at once
  set_by: datastore.WithNamespace on the client for a process-wide value, and firestorebind's per-request resolver for a tenant that varies
  framework_use: one process-wide namespace from data:firestore-runtime-config, and nothing per request
  effect: one configuration value isolates a whole deployment's framework kinds, where the DynamoDB prefix isolates them one declared name at a time
tests: requirement:firestore-test-isolation, which is the same argument reached with the namespace instead of the prefix
what_is_better_than_a_prefix:
  one_value_covers_every_kind: a run cannot half-isolate itself by forgetting a name
  the_service_understands_it: a namespace is part of the key the service stores, unlike a table name, which rule:dynamodb-table-naming records as a string the service reads no structure in
  no_validation_surface: there is no per-name length or character rule to check at startup over an enumerated set, because there is no enumerated set
what_is_worse:
  no_bulk_removal: there is no API that deletes a namespace, so cleanup is a keys-only query per kind followed by a batch delete; a DynamoDB test run deletes its tables instead
  shared_billing_and_quota: two namespaces are one database, so a runaway test run competes with its neighbours for the same per-database write limits; two prefixed DynamoDB table sets do not
  no_per_tenant_isolation_claim: a namespace is a partitioning of keys and not a security boundary, so the framework never presents it as one
what_stays_configurable:
  database: a named database is selected by data:firestore-runtime-config and is a second client rather than a second namespace
  when_to_use_which: a namespace separates runs and tenants inside one database; a named database separates environments that should not share quota
related:
  - requirement:firestore-store
  - requirement:firestore-test-isolation
  - rule:dynamodb-table-naming
  - data:firestore-runtime-config
```
