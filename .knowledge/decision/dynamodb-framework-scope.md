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
  request_reproduction:
    why_framework: the binding layer builds an item and hands it to the driver, so it never sees the request
    seam: decision:dynamodb-observability-seam already captures the HTTP body, and that body is the exact CLI input
    advantage_over_sql: rule:query-reproduction-format has to rebuild a parameterized statement; here there is no placeholder to reconstruct, so reproduction is exact by construction
  offline_doctor_checks:
    available: the generator reports every bound type and every tag error without a network, and returns artifacts without writing a file
    framework_half: endpoint reachability and the deployed-versus-generated diff, because rule:dynamodb-table-naming makes the physical name the framework's
    lands_in: rule:storage-checks, as further PW03xx entries
  seed_and_assert:
    path: decode fixture data with the JSON binder, then EncodeItem; assert by scanning and comparing decoded values
    consequence: the fixture-to-item direction composes from two existing codecs, so requirement:test-data-seeding needs no DynamoDB engine beside system:dbtestify
  paging_cursor:
    available: a driver Key round trips through encoding/json without loss, measured over a 38-digit number, high-byte binary, and a NUL-bearing multi-byte string
    rule: a cursor is a table position and not an authorization, so a signature must cover whatever scoped the query
    fits: the route query binding, which is why this is a web framework's feature rather than a binding layer's
resolved_ask:
  what: a context-resolved form of the generated query function, asked for after v0.2.9
  answered_by: system:tinybind, which carries the client in the context and made the table clause a required part of a declaration
  shape_asked_for: a generation option naming a framework resolver, mirroring the SQL executor resolver
  shape_delivered: resolution inside the runtime entries, with no generation option at all
  why_the_delivered_one_is_better: there is no generated call site to redirect, so the framework installs a client instead of configuring an emitter, and a project using dynamobind without a framework gets the same call shape
  consequence_here: decision:dynamodb-no-runtime-abstraction needs no revision, and the wrapper layer considered as an alternative is unnecessary
sequencing_here:
  now: requirement:dynamodb-typed-queries, plus the four assigned items in whatever order they are wanted
  blocked: any session or auth-state backend, on UpdateTimeToLive
  blocked: evolving a table, on UpdateTable
related:
  - requirement:dynamodb-store
  - requirement:dynamodb-typed-queries
  - system:tinybind
  - system:tinygodriver-dynamodb
```
