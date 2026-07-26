---
id: rule:framework-owned-tables
type: rule
title: Framework Owned Tables
---
A table created by framework middleware carries the `popcornwave_` prefix and reaches the database through its own migration file, never through startup.

```yaml
naming:
  prefix: popcornwave_
  reason: an application reading its own schema can tell at a glance which tables it does not own
current_tables:
  popcornwave_session: plugin/session/rdb login sessions
  popcornwave_authstate: contrib/authstate/sqlite single-use ceremony records
  popcornwave_auth_allowlist: plugin/auth pre-registration for policy:oidc-admission registered mode
migrations:
  location: the application migration directory, beside application migrations
  naming: "{version}_init_popcornwave_{capability}.sql"
  reserved_versions: below 00010, so a new framework table never renumbers application migrations
  source: the owning package publishes the exact file content, and a repository test fails when a copy drifts
  scaffolding: api:cli-init writes them from the selected authentication mode once decision:authentication-bootstrap-strategy modes are implemented
startup:
  action: verify only
  missing_table: refuse to serve and name the migration file and the command that applies it
  shape_check: validate the column layout of an existing table
  forbidden: creating, altering, or dropping a table while serving
rules:
  - one migration file per owning package
  - a package never writes to a table another package owns
  - application migrations never modify a popcornwave_ table
  - records that expire without being consumed are swept, because expiry is logical
related:
  - api:session-store
  - api:cli-session-schema
  - policy:migration-safety
```
