---
id: api:cli-schema-init
type: api
title: pw schema-init
---
pw schema-init initializes an empty application database from version-control-owned SQL without claiming migration support.

```yaml
usage: pw schema-init
source:
  directory: dbschema
  files: lexical-order SQL files
database:
  configuration: decision:config-driven-database
  lifecycle: command opens and closes its own connection
behavior:
  - load the effective runtime TOML configuration
  - validate the configured database driver and DSN
  - execute schema files transactionally when the driver supports transactional DDL
  - stop on the first error and return nonzero
rules:
  - schema sources are handwritten and committed
  - initialization targets a new or explicitly compatible database
  - SQL should be idempotent where practical
  - redact credentials from output and errors
non_goals:
  - schema version history
  - upgrade or downgrade planning
  - destructive migration
  - initial row loading; api:cli-seed owns data:seed-dataset
future: replace or extend with a versioned migration system
```
