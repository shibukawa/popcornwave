---
id: requirement:dynamodb-typed-queries
type: requirement
title: Declared DynamoDB Queries
---
A .pw.dynamo declaration generates one named function per access pattern, so a call site names no attribute, no placeholder, and no expression.

```yaml
owner: system:tinybind, from v0.2.9
closes: the half requirement:dynamodb-generation left open, where the codec typed the item and the read path was still a key-condition string
source_file:
  suffix: .pw.dynamo, set through the generator DynamoTemplatePattern option, the same way .pw.sql and .pw.html are branded
  location: beside the package, discovered per directory listed under generate.dynamo
  outer_form: reused from .pw.sql - an export statement, a typed parameter list, a result type after a colon, and a braced body
  no_package_line: unlike .pw.sql, a declaration file declares no package; the directory it sits in is the package, found on implementation 2026-08-01
  body: a table clause and a key clause rather than SQL text, in either order, separated by ";" when on one line
  example: "export statement ReadingsSince(sensor: Sensor, from: int64): dynamo.many<Reading> { table readings; key sensor = {sensor} and at > {from} }"
  export_keyword: must agree with the name's own casing, since Go decides visibility by the name; either without the other is an error rather than a silent rename
table_clause:
  required: in every statement, so one declaration form yields one signature
  effect: the generated function takes no table parameter
  checked: against the DynamoDB name rule at generation, and against decision:dynamodb-table-registry so a declaration cannot name a table requirement:dynamodb-migration will not create
  deployed_name: mapped at run time by rule:dynamodb-table-naming, so the declaration states the logical fact and the deployment states the physical one
  why_the_statement_and_not_the_type: the same struct can be stored in a test table and a production one, so binding a table to a type would assert something untrue; an access pattern names exactly one
result_type:
  chooses: the request shape, not a row count, because a Query always returns many
  "dynamo.page<T>": one request, returning Page[T] with Count, ScannedCount, and LastEvaluatedKey
  "dynamo.many<T>": an iterator over every page
  reason: the request count stays the author's decision, per system:tinybind driver passthrough
generation_checks:
  owner: system:tinybind
  attribute_names: every attribute named must exist on the bound type's dynamo tags, so a renamed tag fails generation instead of failing in production
  key_clause: the partition key with equality, plus at most one sort key predicate from =, <, <=, >, >=, between, and begins_with
  non_key_attribute: an error naming the clause it would belong in
  parameter_types: checked against how the tag stores the attribute, S or N
  reserved_words: every attribute is aliased unconditionally as #k0 and its siblings, so none of the 573 reserved words can reach an expression literally
  value: this is the check flow:sql-generation cannot make, because there the schema is not in the source; here the tags are the schema
generated:
  per_declaration: one exported function, plus a constant key condition and a constant attribute-name map
  values: built per call from the typed parameters through the same encoders the codec uses
  file: one per package, named through the generator DynamoQueryName option so the output carries the Popcorn Web suffix rather than the tinybind default
call_site_shape:
  form: "records.ReadingsSince(ctx, sensor, from)"
  signature: context, the declared parameters, then variadic driver query options; the generated names and values are appended last, so a caller option cannot replace the condition
  absent: the table, which the declaration names, and the client, which the context carries
  parity: identical in shape to a flow:sql-generation function, and reached without the executor resolver that path needs
  how: system:tinybind resolves both inside the runtime entry, so there is no generated call site for a framework to redirect
counts_as_usage:
  fact: a declaration is a use of its result type, because the generated function instantiates the runtime query with it
  effect: a package whose only DynamoDB use is a declaration still gets a codec, so requirement:dynamodb-generation needs no separate call to trigger one
scope:
  in: key condition, limit, scan direction, consistent read
  out: filter, projection, condition, and update expressions, which join the same declaration when system:tinybind implements them
  index: a declared query runs against the table's own keys until secondary index tags exist
  escape_hatch: the string key-condition form stays available and stays unchecked, so a call using it aliases its own reserved words
acceptance:
  - a .pw.dynamo declaration in a generate.dynamo directory produces a function whose call site names no attribute, no client, and no table
  - a handler reaching a declared query without the api:dynamo-package middleware gets a named no-client error rather than a panic
  - renaming a dynamo tag without editing the declaration fails api:cli-generate rather than the first request
  - a declaration naming a non-key attribute fails with the clause named
  - a .pw.dynamo outside every generate.dynamo entry is reported once, naming the path and the key
  - generated output carries the Popcorn Web file suffix, with no post-generation rename
non_goals:
  - a Popcorn Web query language of its own; the declaration is system:tinybind's
  - filter or update expression generation, which is upstream's to add
  - single-table dispatch, which system:tinybind declines outright
```
