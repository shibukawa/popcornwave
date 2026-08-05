---
id: requirement:dynamodb-store
type: requirement
title: DynamoDB Store
---
DynamoDB is an optional store a project adds beside its relational database, not a fourth answer to requirement:database-engine-selection.

```yaml
motivation:
  - system:tinybind dynamobind and system:tinygodriver-dynamodb together make a typed DynamoDB path that builds under TinyGo
  - DynamoDB is not database/sql, so it shares no part of data:database-connection-set, api:database-selection, api:transaction-runner, or system:goose
  - a project keeping sessions in SQLite and events in DynamoDB is the expected shape rather than an exception
placement:
  section: middleware.dynamo, its own data:dynamodb-runtime-config, unrelated to middleware.rdb
  combinations: rdb only, dynamo only, and both are all valid
  engine_question: requirement:database-engine-selection keeps naming one SQL engine; DynamoDB is a separate capability answer
  restated_non_goal: its "more than one database in a project" exclusion means more than one relational database, and never excluded a second kind of store
rejected_alternative:
  form: dynamo as a project.database value beside sqlite, postgres, and mysql
  why_not:
    - it would make plugin/session/rdb, system:goose, .pw.sql, and api:database-selection unreachable for that project
    - rule:rdb-dsn-resolution resolves a DSN to a database/sql opener, and this client has no such opener to resolve to
    - a project needing both stores could not say so
surfaces:
  package: api:dynamo-package
  configuration: data:dynamodb-runtime-config
  generation: requirement:dynamodb-generation
  queries: requirement:dynamodb-typed-queries
  work_allocation: decision:dynamodb-framework-scope
  schema: requirement:dynamodb-migration
  tests: requirement:dynamodb-test-isolation
  observability: decision:dynamodb-observability-seam
  runtime_abstraction: none, per decision:dynamodb-no-runtime-abstraction
bounded_by_the_stack:
  - no transactions, PartiQL, Streams, or DAX; the driver does not implement them, which is what decision:dynamodb-auth-compensating-registration works around
  - no UpdateTable, so no table is altered in place and an index missing at creation stays missing
  - no secondary index tags, because system:tinybind defers the gsi tag option; this bounds a generated definition, and a handwritten one may declare an index at creation, which requirement:dynamodb-auth-stores does
  effect: these bound requirement:dynamodb-migration by what exists upstream, not by a Popcorn Wave choice
tinygo:
  buildable: the client and dynamobind both build under TinyGo, so no part of this store needs decision:migration-execution-split delegation
  contrast: system:goose is host-only, which is the whole reason the SQL path has a delegated backend
  size: the context client path costs about 37 KB on a wasip1 build, measured upstream; a size-critical program calls the driver with the generated methods and links none of it
acceptance:
  - a project with a dynamo section and no rdb section starts, applies its schema, and serves
  - a project with both sections opens both and reports both in policy:startup-summary
  - an application that never imports api:dynamo-package links no DynamoDB code and gains no configuration key
  - the same application source builds and runs under host Go and under TinyGo
expected_upstream:
  version_tag: optimistic locking from a version tag, proposed in system:tinybind; nothing here changes when it lands
  ttl_tag: an expiry attribute in the generated table definition, blocked until the driver can apply a TTL, per system:tinygodriver-dynamodb
non_goals:
  - a portable key-value facade over DynamoDB and any other store
  - routing a generated .pw.sql query to DynamoDB
  - changing the relational default of decision:tinygo-storage-direction
  - single-table design; system:tinybind states it as a non-goal, so one struct owns one table here too
  - the session backend itself, which is requirement:dynamodb-session-store and depends on this rather than belonging to it
  - the authentication backend, which is requirement:dynamodb-auth-backend and depends on this the same way
```
