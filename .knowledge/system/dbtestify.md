---
id: system:dbtestify
type: system
title: dbtestify
---
dbtestify is the external owner of dataset-driven database seeding and assertion logic consumed by requirement:test-data-seeding.

```yaml
module: github.com/shibukawa/dbtestify
source: https://github.com/shibukawa/dbtestify
packages:
  core: github.com/shibukawa/dbtestify
  assertdb: github.com/shibukawa/dbtestify/assertdb
  httpapi: github.com/shibukawa/dbtestify/httpapi
  cli: github.com/shibukawa/dbtestify/cmd/dbtestify
core_surface:
  - ParseYAML(io.Reader) (*DataSet, error)
  - Seed(ctx, DBConnector, *DataSet, SeedOpt) error
  - Assert(ctx, DBConnector, *DataSet, AssertOpt) (bool, []AssertTableResult, error)
  - DBConnector interface over TableNames, PrimaryKeys, Insert, Delete, Upsert, Truncate, DB
  - NewDBConnector(ctx, "postgres|mysql|sqlite://dsn") (DBConnector, error)
  - Operation clear-insert, insert, upsert, delete, truncate
  - MatchStrategy exact, sub
  - DumpDiffCLICallback(showTableName, quiet bool)
consumed:
  - core only
not_consumed:
  assertdb: binds concrete *testing.T and t.Context, and opens its own pool; decision:testutil-testing-interface rules it out
  httpapi: out of scope for requirement:test-data-seeding
  cli: replaced by api:cli-seed so one runtime configuration source stays authoritative
added_for_popcornwave:
  released: v0.2.0 and v0.3.0
  rationale: decision:dbtestify-integration
  surface:
    - NewDBConnectorFromDB(db *sql.DB, dialect Dialect) (DBConnector, error)
    - Dialect, ParseDialect(name), SplitSource(source)
    - DiffFormat, FormatTableDiff, DumpDiffCallback(io.Writer, DiffFormat)
    - github.com/shibukawa/dbtestify/connect owns NewDBConnector and the driver imports
    - Executor, NewDBConnectorFromTx, NewDBConnectorFromExecutor
    - assertdb SeedDataSetDB, AssertDBWithDB, SeedDataSetTx, AssertDBTx
  core_dependencies_after: fatih/color and goccy/go-yaml only
  breaking:
    - v0.2.0 moved dbtestify.NewDBConnector to connect.NewDBConnector
    - v0.3.0 DBConnector operations take an Executor instead of a *sql.Tx
```
