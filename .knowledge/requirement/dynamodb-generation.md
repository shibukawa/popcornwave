---
id: requirement:dynamodb-generation
type: requirement
title: DynamoDB Code Generation
---
api:cli-generate runs the system:tinybind dynamobind mode over a new generate.dynamo purpose, so a dynamo-tagged struct produces its item codec, key builder, and table definition beside its source.

```yaml
purpose_key:
  name: generate.dynamo
  reads: Go sources carrying dynamo struct tags, the dynamobind call sites that direct which half of each codec is emitted, and the .pw.dynamo query declarations of requirement:dynamodb-typed-queries
  emits: "{source-base}_pw_gen.go beside each source, one query file per package, plus decision:dynamodb-table-registry"
  required: no, unlike the four original keys; a missing key means the empty list, because a project written before this purpose could not have named it
  precedent: generate.pages took the same optional shape for the same reason, per decision:explicit-generation-sources
  disjointness: a directory may also be a generate.handlers entry, because a tagged record commonly lives beside the handler that stores it
why_a_new_purpose:
  not_queries: generate.queries reads .pw.sql, and dynamobind reads Go type declarations and call sites; one scanner would have to do both
  not_handlers: generate.handlers reads route registrations and pw.Parse sites, and a record package has neither
generated_per_type:
  - EncodeItem, when a discovered call writes the type
  - DecodeItem, when a discovered call reads it
  - ItemKey and the table constructor, whenever a partitionkey tag exists, which system:tinybind emits without waiting for a discoverable use
  - measured_exception: the artifact path api:cli-generate uses skips a type with no discovered call, so a tagged declaration nobody stores or loads produces no file at all; the file-writing entry point upstream is the one that emits without a use
  - the compile-time assertions that make a stale generated file a build failure
usage_direction:
  owner: system:tinybind
  consequence: adding the first dynamobind read call to an existing project changes generated output, and api:cli-check reports it
  first_run: a clean checkout finds every call, because the argument type resolves before the codec exists
validation_surfaced_by_pw:
  - an unknown dynamo tag option is a generation error naming the field and the option
  - a field carrying dynamodbav without dynamo is a generation error naming both spellings
  - a duplicate attribute, a second partitionkey, a sortkey without one, and a non-key-typed key field are generation errors
  - two types resolving to one declared table name, per rule:dynamodb-table-naming
  rule: Popcorn Web reports these; it adds no second validator, mirroring how flow:sql-generation defers to system:tinybind
dialect: none; unlike flow:sql-generation there is no engine variant to pass, so project.database is not read for this purpose
read_path: requirement:dynamodb-typed-queries owns it; this requirement covers the item path only, which is where the codec, the key builder, and the table definition come from
feature_suppression:
  available: system:tinybind can suppress the table definition as the named feature item-table, for a project whose tables are owned by CloudFormation or Terraform
  interaction: decision:dynamodb-table-registry has nothing to collect once it is suppressed, so requirement:dynamodb-migration reports that the schema source is empty rather than reporting that every table is missing
  rule: suppressing it is a project decision recorded in configuration, never a silent consequence of an empty directory
scoping_mechanism:
  html_and_sql: the generation request carries a per-directory pattern, so an unlisted template is never parsed
  dynamo: the request carries no such field, so api:cli-generate runs an unlisted directory against a copy of the generator whose declaration glob matches nothing
  why_not_filter_afterwards: filtering artifacts would still have read and type-checked the declaration, which is what the purpose exists to prevent
  found: implementation 2026-08-01
determinism: same input yields byte-identical output, and --check fails on a difference like every other purpose
acceptance:
  - a tagged struct in a generate.dynamo directory produces a codec that round trips through system:tinygodriver-dynamodb without a call site naming an attribute string
  - a struct outside every generate.dynamo entry generates nothing and is reported once, naming the path and the key
  - regenerating an unchanged project rewrites no file
  - a project with no generate.dynamo entry loads and generates exactly as before
non_goals:
  - generated update or condition expressions, which system:tinybind defers
  - generated secondary index keys, which system:tinybind defers
  - single-table design, which system:tinybind declines outright
```
