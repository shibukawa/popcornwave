---
id: api:cli-seed
type: api
title: pw seed
---
pw seed loads data:seed-dataset files into the configured application database through the same effective runtime configuration used by the server.

```yaml
usage: pw seed [name...]
flags:
  --dir: override the testdata/seed directory
arguments:
  - each name is a data:seed-dataset path relative to the seed directory
  - no argument applies every .yaml file in the seed directory in lexical order
execution:
  model: decision:host-tools-target-runtime
  mechanism: go run <data:project-config main> --pw-seed=<resolved paths>
  path_encoding: os.PathListSeparator joined absolute paths
  framework_action: same dispatch position as api:cli-schema-init
  engine: decision:dbtestify-integration local_boundary
behavior:
  - load the effective runtime TOML configuration
  - validate the configured runtime like api:cli-schema-init does
  - require middleware.rdb.enabled and fail otherwise
  - open the framework-owned pool through decision:config-driven-database
  - resolve the system:dbtestify dialect from the rdb DSN scheme
  - apply datasets in argument order, each in its own transaction
  - stop on the first error and return nonzero
  - close runtime resources through api:application-lifecycle
rules:
  - schema must already exist; api:cli-schema-init is a separate command and is not run implicitly
  - seeding is destructive by default because clear-insert truncates targeted tables
  - the command targets development and test databases, never a shared production database
  - redact credentials from output and errors
  - unknown dataset names are errors
non_goals:
  - assertion from the CLI; comparison belongs to api:test-seed
  - automatic seeding during api:cli-dev startup
  - dataset generation from a live database
future:
  - optional api:cli-dev integration that reseeds on demand
```
