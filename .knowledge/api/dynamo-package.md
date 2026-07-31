---
id: api:dynamo-package
type: api
title: database/dynamo Package
---
Importing github.com/shibukawa/popcornwave/database/dynamo registers the DynamoDB configuration binding, opens the client, and installs it into every request context; the operations a handler calls are system:tinybind's, reached without naming the package.

```yaml
import: github.com/shibukawa/popcornwave/database/dynamo
import_style:
  form: a normal import, following decision:import-registered-session-plugins for the registration half
  effect_of_importing: data:dynamodb-runtime-config appears in the configuration schema and the middleware registers itself from init
  effect_of_not_importing: no configuration key, no middleware, no linked driver
  boundary: this package registers nothing into the rule:rdb-dsn-resolution engine registry, because a DynamoDB endpoint is not an rdb DSN
middleware:
  installed_by: the init registration, as one more entry in the middleware chain
  per_request: install the client into the request context with the system:tinybind client setter, carrying the rule:dynamodb-table-naming resolver as its option
  effect: every dynamobind entry and every generated query function reads both from the context, so no handler passes a client or a deployed table name
  missing_client: system:tinybind returns a named no-client error rather than panicking, so a handler that ran without the middleware fails as an ordinary error
surface:
  - Migrate(context.Context, ...MigrateOption) (Result, error)
  - Plan(context.Context, ...MigrateOption) ([]TableChange, error)
  - RegisterTable(declared string, def func(name string) dynamodb.TableDefinition)
  - WithTableResolver(fn) as a startup option, for a deployment rule:dynamodb-table-naming configuration cannot express
deliberately_absent:
  client_accessor: system:tinybind already exposes a client-from-context escape hatch, so re-exporting it would be one name for one thing twice
  table_accessor: superseded; the resolver runs inside the runtime entry, so no call site resolves a name
  operation_wrappers: none, per decision:dynamodb-no-runtime-abstraction; the earlier plan to add them existed only to fill in a client and a table, and neither argument survives
usage:
  item: "dynamobind.Load[Reading](ctx, \"reading\", r.ItemKey())"
  query: "records.ReadingsSince(ctx, sensor, from)"
  parity: a declared query takes context and its parameters, exactly as a flow:sql-generation function does
  remaining_argument: an item operation still names a table, because it has no declaration to read one from; a declared query names neither
lifecycle:
  startup: construct the client, verify credentials and region, and fail before serving when either is missing
  readiness: ListTables with a bounded limit answers the readiness probe, since the driver ships no ping
  shutdown: Close the client through api:application-lifecycle, unless decision:dynamodb-observability-seam supplied the HTTP client, which the driver then leaves alone
migration:
  entry: Migrate and Plan implement requirement:dynamodb-migration in process
  names: resolved through the same rule:dynamodb-table-naming resolver, built from configuration rather than from a request, so the CLI and a handler address one table
  parity: the same code path serves api:cli-migrate through decision:dynamodb-table-registry
constraints:
  - no operation wrapper, error type, or option type of its own
  - no transaction surface; the driver has none
  - the client is fixed at startup and is neither replaced nor reopened per request
  - a test or a second region installs a second context, not a second signature
  - credentials and endpoint never reach a log, an error, or policy:startup-summary unredacted
```
