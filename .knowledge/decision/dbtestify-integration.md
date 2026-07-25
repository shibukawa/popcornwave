---
id: decision:dbtestify-integration
type: decision
title: dbtestify Integration Boundary
---
Popcorn Wave delegates seed and assertion logic to the system:dbtestify core package and requires upstream API additions instead of reimplementing or vendoring that logic.

```yaml
status: accepted
scope: requirement:test-data-seeding
rationale:
  - dataset parsing, operation ordering, row matching, and diff reporting are already solved upstream
  - a second implementation would fork dataset semantics between api:cli-seed and api:test-seed
  - both owners are the same maintainer, so upstream changes are cheaper than a local fork
upstream_changes:
  status: released as system:dbtestify v0.2.0
  connector_from_pool:
    add: NewDBConnectorFromDB(db *sql.DB, dialect Dialect) (DBConnector, error)
    contract: dbtestify never closes or reconfigures a pool it did not open
    reason:
      - decision:config-driven-database makes the framework the sole owner of *sql.DB
      - api:test-run in-memory SQLite is unreachable from a second independently opened pool
      - a second pool would double pool limits and bypass validated rdb configuration
  driver_import_split:
    move: NewDBConnector and the blank driver imports to github.com/shibukawa/dbtestify/connect
    keep_in_core: ParseYAML, Seed, Assert, DBConnector, NewDBConnectorFromDB, Dialect, ParseDialect, SplitSource, DataSet, SeedOpt, AssertOpt, result types
    core_dependencies: fatih/color and goccy/go-yaml only
    caller_impact: stale dbtestify.NewDBConnector call sites fail to compile; migration is an import path change
    rejected_variant:
      idea: keep NewDBConnector in core and register drivers from a blank-imported subpackage
      reason: it compiles and then fails at run time with unknown driver, which hides the break until a test runs
    reason:
      - core previously forced mattn/go-sqlite3 and therefore CGo on every consumer
      - decision:sqlite-backend-selection requires the sqlite driver to come from system:tinygodriver
      - decision:server-sql-support-tier must not be widened by transitive pgx and mysql imports
  exported_dialect:
    add: Dialect string enum for postgres, mysql, sqlite plus ParseDialect
    reason: api:test-seed resolves the dialect from the configured rdb DSN scheme, not from a connection string
  writer_diff_formatter:
    add: DiffFormat, FormatTableDiff, and DumpDiffCallback(io.Writer, DiffFormat)
    preserved: DumpDiffCLICallback keeps its colored os.Stdout behavior
    reason: assertion failures must reach TestingT output instead of os.Stdout
consequences:
  - popcornwave gains build dependencies on the dbtestify core package and goccy/go-yaml only
  - popcornwave gains no CGo requirement and no pgx or mysql driver dependency
  - pool ownership stays with api:rdb-middleware and api:test-run
  - dataset semantics stay defined by system:dbtestify and documented by data:seed-dataset
rejected:
  separate_popcornwave_module:
    idea: isolate seeding in its own Go module to contain dbtestify dependencies
    reason: api:cli-seed and api:test-run options must live in the framework module to stay first class
  accept_dependencies_as_is:
    idea: depend on unmodified dbtestify core
    reason: breaks CGo-free builds and contradicts decision:sqlite-backend-selection
local_boundary:
  package: github.com/shibukawa/popcornwave/internal/dbseed
  role: the only place popcornwave imports dbtestify, shared by api:cli-seed and api:test-seed
  surface: DefaultDir, Extension, Resolve, Dialect, ResolveDialect, Apply, Assert
added_dependencies:
  direct: github.com/shibukawa/dbtestify v0.2.0
  indirect: fatih/color, goccy/go-yaml, mattn/go-colorable
  absent: pgx, go-sql-driver/mysql, and any new CGo requirement
```
