---
id: decision:dynamodb-framework-scope
type: decision
title: What The DynamoDB Binding Layer Left To The Framework
---
system:tinybind v0.2.9 answered a downstream request by keeping three items, sending two to the driver, and assigning four to Popcorn Wave because the seams they need already exist.

```yaml
status: accepted upstream 2026-07-31, recorded here as the work allocation
upstream_kept:
  typed_queries: shipped; requirement:dynamodb-typed-queries consumes it
  optimistic_locking: proposed, a version tag making a write conditional on the version it read
  ttl_attribute: proposed and blocked, because the driver cannot apply a TTL
sent_to_the_driver:
  ranked: by how much one change unlocks downstream
  first: UpdateTimeToLive, which unlocks the session and auth-state backends and the TTL half of requirement:dynamodb-migration
  second: UpdateTable, without which a table carrying a secondary index can be created and never evolved
  third: transactions, larger than the other two together
  tracked_in: system:tinygodriver-dynamodb
assigned_here:
  all_four_specified: 2026-08-01
  request_reproduction:
    why_framework: the binding layer builds an item and hands it to the driver, so it never sees the request
    specified_by: data:dynamodb-request-record and rule:dynamodb-reproduction-format
  offline_doctor_checks:
    available: the generator reports every bound type and every tag error without a network, and returns artifacts without writing a file
    framework_half: endpoint reachability and the deployed-versus-generated diff, because rule:dynamodb-table-naming makes the deployed name the framework's
    specified_by: rule:storage-checks, as further PW03xx entries
  seed_and_assert:
    path: decode fixture data with the JSON binder, then encode through the generated item codec; assert by reading and comparing decoded values
    specified_by: requirement:dynamodb-test-data
  paging_cursor:
    available: a driver Key round trips through encoding/json without loss, measured over a 38-digit number, high-byte binary, and a NUL-bearing multi-byte string
    specified_by: requirement:dynamodb-page-cursor
    signature_question: upstream cautions that a signature must cover whatever scopes the query; that requirement answers it by keeping the scope in the key condition, where a forged cursor cannot reach past it, and by naming the filter case that would change the answer
beyond_the_four:
  auth_state: requirement:contrib-auth-state-dynamo, which was never assigned because it is a Popcorn Wave contract with no upstream half at all
  auth_backend: requirement:dynamodb-auth-backend, the same shape; plugin/auth is entirely a Popcorn Wave contract
resolved_ask:
  what: a context-resolved form of the generated query function, asked for after v0.2.9
  answered_by: system:tinybind, which carries the client in the context and made the table clause a required part of a declaration
  shape_asked_for: a generation option naming a framework resolver, mirroring the SQL executor resolver
  shape_delivered: resolution inside the runtime entries, with no generation option at all
  why_the_delivered_one_is_better: there is no generated call site to redirect, so the framework installs a client instead of configuring an emitter, and a project using dynamobind without a framework gets the same call shape
  consequence_here: decision:dynamodb-no-runtime-abstraction needs no revision, and the wrapper layer considered as an alternative is unnecessary
sequencing_here:
  now: requirement:dynamodb-typed-queries, plus the four assigned items in whatever order they are wanted
  unblocked: the session and auth-state backends, which read as blocked on UpdateTimeToLive above; decision:dynamodb-operational-configuration withdrew that ask, so neither needs a driver change
  blocked: evolving a table, on UpdateTable
  not_blocked_either: requirement:dynamodb-auth-backend, which decision:dynamodb-auth-compensating-registration keeps off the ranked-third transaction item
related:
  - requirement:dynamodb-store
  - requirement:dynamodb-typed-queries
  - system:tinybind
  - system:tinygodriver-dynamodb
```
