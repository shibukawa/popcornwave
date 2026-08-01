---
id: requirement:dynamodb-test-data
type: requirement
title: DynamoDB Seeding And Assertion
---
The data:seed-dataset file format loads items into a DynamoDB table and serves as the expected result, so a test written against this store reads like one written against a relational one.

```yaml
goals:
  - one dataset format across both stores, so a project using both learns one thing
  - remove handwritten PutItem fixtures from tests and development bootstrap
  - make stored state assertable without a handwritten scan and compare
format:
  reused: data:seed-dataset as written, where a top-level key is a table name and its value is a list of item maps
  table_name: the declared name, resolved through rule:dynamodb-table-naming, so a dataset is portable across environments and test prefixes
  values: decoded into the bound Go type through the generated codec, so a dataset states application values and never an attribute map
  why_not_a_second_format: the shape is a table and a list of records, which is what both stores are being asked for
operations:
  clear-insert: remove every item of the table, then put; the default
  insert: put, failing when an item with that key exists
  upsert: put, replacing
  delete: delete by key
  truncate: remove every item and put nothing
  removal_cost: clearing scans the table, which is acceptable only because a seeded test table is small; a dataset is not a production tool
  no_transaction: each operation is its own request, so a partially applied dataset is possible and a failure reports how far it got
assertion:
  compare: read every item of the named table, decode through the codec, and match against the dataset
  keys: taken from the generated table definition rather than from the live schema, since the definition is the schema
  exact: fails on an item present only in the store
  sub: ignores one
  matchers: the data:seed-dataset set, applied to decoded Go values rather than to attribute values
  absent_attributes: an attribute the dataset omits is ignored, matching the relational rule
isolation:
  mechanism: requirement:dynamodb-test-isolation, which gives each test server its own table prefix
  contrast: api:test-seed leans on the api:test-run test transaction to undo per-test writes, and this store has no transaction to lean on
  consequence: a dataset is the baseline and re-seeding is how a test resets, since nothing rolls back
  re_seed: Seed may run again mid-test, and clear-insert makes that a reset rather than an accumulation
engine:
  parser: the data:seed-dataset format, shared with system:dbtestify
  executor: this store's own, because system:dbtestify speaks database/sql and nothing here does
  path: decode the dataset with the JSON binder, encode through the generated item codec, and batch the writes
  reason_it_composes: both codecs already exist, so no fixture runner has to be written for this store
surface:
  cli: api:cli-seed against the configured store
  test: api:test-seed, whose existing options gain this store without a new name
  selection: a dataset naming a DynamoDB table reaches this executor; one naming a relational table reaches the other
acceptance:
  - a dataset applied by the CLI and by a test produces the same items
  - the same file works as a seed input and as an expected result
  - an assertion mismatch reports a per-item diff and continues, matching api:test-seed
  - a dataset for a table with no generated definition fails naming the table, not the item
  - two parallel test servers seeding the same dataset do not observe each other
  - a value that round trips through the codec compares equal to what the dataset declared
non_goals:
  - production data loading
  - a dataset that creates or alters a table
  - snapshotting a live table into a dataset
  - clearing a table by any means cheaper than a scan, which would need a capability the driver does not expose
```
