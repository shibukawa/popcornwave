---
id: requirement:test-data-seeding
type: requirement
title: Test Data Seeding and Comparison
---
Popcorn Wave provides one dataset format that seeds a database from the CLI and from unit tests and also serves as the expected result for database assertions.

```yaml
goals:
  - remove handwritten INSERT fixtures from tests and development bootstrap
  - keep one dataset definition shared by api:cli-seed and api:test-seed
  - make database state assertable without hand-rolled SELECT comparison
  - keep the framework module free of CGo and extra SQL driver dependencies
scope:
  dataset: data:seed-dataset
  cli: api:cli-seed
  test: api:test-seed
  engine: system:dbtestify through decision:dbtestify-integration
constraints:
  - decision:config-driven-database keeps pool ownership with the framework
  - decision:sqlite-backend-selection keeps the sqlite driver in system:tinygodriver
  - decision:server-sql-support-tier bounds which databases are supported
  - requirement:database-migration stays the only owner of schema creation
  - concept:public-package-boundaries keeps seeding out of the request path
acceptance:
  - pw seed applies a named dataset to the configured database and exits nonzero on failure
  - pw seed with no argument applies every dataset in lexical order
  - pw seed fails clearly when middleware.rdb.enabled is false
  - TestRun with WithMigrations and WithSeed starts with schema installed then rows loaded
  - Server.Seed reloads a dataset mid-test
  - Server.AssertDB passes on matching state and reports a per-table diff through Errorf on mismatch
  - decision:testutil-testing-interface TestingT gains Errorf and existing *testing.T callers still compile
  - AssertDB honors exact and sub match strategies and the any, null, notnull, regexp, and currentdate matchers
  - a dataset file works unchanged as both seed input and expected result
  - the framework module builds with CGO_ENABLED=0
  - no test opens a second pool against the api:test-run database
non_goals:
  - schema migration or version history
  - production data loading
  - dataset generation from a live database
  - the dbtestify HTTP API and standalone CLI
```
