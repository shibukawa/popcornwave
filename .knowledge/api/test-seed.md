---
id: api:test-seed
type: api
title: Test Seeding and Assertion
---
testutil seeds an api:test-run server from data:seed-dataset files and compares the resulting database state against expected datasets.

```yaml
package: github.com/shibukawa/popcornweb/testutil
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
  - install the api:test-run migration schema
  - apply WithSeed files in declared order
  - begin the api:test-run WithTransaction test transaction
  - start the HTTP server
transaction_interaction:
  with_seed: commits before the test transaction opens, so datasets are the shared baseline and only per-test writes roll back
  server_methods: Seed and AssertDB run on the test transaction when one is active, and on the pool otherwise
  mechanism: pwruntime TransactionScope Tx feeds the system:dbtestify executor connector
  under_transaction:
    - seeded rows are visible to requests and disappear with the rollback
    - assertions observe request writes before any commit
    - no second connection is taken from a pool the transaction already holds
connector:
  pool: the api:test-run owned *sql.DB; no second pool is opened
  construction: decision:dbtestify-integration NewDBConnectorFromDB, or NewDBConnectorFromTx inside a test transaction
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
