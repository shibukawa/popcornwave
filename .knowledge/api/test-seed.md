---
id: api:test-seed
type: api
title: Test Seeding and Assertion
---
testutil seeds an api:test-run server from data:seed-dataset files and compares the resulting database state against expected datasets.

```yaml
package: github.com/shibukawa/popcornwave/testutil
surface:
  - WithSeed(files ...string) RunOption
  - WithSeedDir(directory string) RunOption
  - (*Server).Seed(t TestingT, files ...string)
  - (*Server).AssertDB(t TestingT, files ...string)
shape:
  seed: RunOption so the database is ready before the first request
  assert: explicit Server method so comparison timing is caller controlled
run_option_order:
  - open the copied pool through api:test-run
  - apply WithSchemaDir
  - apply WithSeed files in declared order
  - start the HTTP server
connector:
  pool: the api:test-run owned *sql.DB; no second pool is opened
  construction: decision:dbtestify-integration NewDBConnectorFromDB
  dialect: resolved from the copied data:middleware-runtime-config rdb DSN scheme
failure_reporting:
  interface: decision:testutil-testing-interface
  requires: Errorf added to TestingT for this API
  seed: TestingT Fatalf; a failed seed invalidates the test
  assert: TestingT Errorf per mismatching table with an uncolored diff, then the test continues
  disabled_rdb: Fatalf when middleware.rdb.enabled is false
  context: caller context or context.Background; TestingT exposes no Context
rules:
  - Seed may run again mid-test to reset state between phases
  - AssertDB may run more than once against different expected datasets
  - AssertDB compares committed state, so streaming or auto-transaction responses must be complete first
  - dataset paths resolve through data:seed-dataset test root resolution
  - no method mutates the seed directory or writes datasets back
non_goals:
  - automatic assertion on Server.Close
  - HTTP response body assertion
  - snapshot capture of a live database into a dataset
```
