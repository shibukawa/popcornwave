---
id: data:seed-dataset
type: data
title: Seed Dataset File
---
A seed dataset is a handwritten YAML file under testdata/seed that describes table rows for both database seeding and expected-state assertion.

```yaml
location:
  directory: testdata/seed
  extension: .yaml
  root_resolution:
    cli: project root of data:project-config
    test: package directory of the running go test binary
  argument: path relative to the seed directory; .yaml is appended when omitted
  override:
    cli: api:cli-seed --dir
    test: api:test-seed WithSeedDir
format:
  owner: system:dbtestify ParseYAML
  body: top-level key is a table name, value is a list of row maps
  reserved_keys:
    _operation: map of table name to clear-insert, insert, upsert, delete, or truncate
    _match: map of table name to exact or sub
    _tag: per-row string or string list used by include and exclude tag filters
  defaults:
    operation: clear-insert
    match: exact
assertion_matchers:
  form: expected value is a list whose first element names the matcher
  values:
    - "[any]"
    - "[null]"
    - "[notnull]"
    - "[regexp, pattern]"
    - "[currentdate, duration]"
  currentdate_default_duration: 1m
semantics:
  seed:
    - truncate every targeted table first
    - insert, upsert, or delete rows per table operation
    - one transaction for the whole dataset
  assert:
    - compare row by row on primary keys resolved from the live schema
    - columns absent from the dataset are ignored
    - exact fails on rows present only in the database; sub ignores them
rules:
  - dataset files are handwritten and committed
  - the same file may serve as a seed input and as an expected result
  - datasets never contain DDL; api:cli-schema-init owns schema creation
  - primary keys must exist in the database or the row is reported as wrongDataSet
  - datasets carry no credentials or environment-specific connection data
non_goals:
  - CSV or spreadsheet dataset sources
  - schema migration
  - generated or scaffolded dataset files
```
