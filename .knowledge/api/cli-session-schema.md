---
id: api:cli-session-schema
type: api
title: pw session schema
---
The CLI prints, checks, or applies only the selected RDB session plugin schema; application migrations remain outside Popcorn Web.

```yaml
status: not implemented; rule:framework-owned-tables carries plugin schema as migration files instead
usage: "pw session schema --config <runtime.toml> [--check | --apply] [--output <file>]"
default:
  action: print deterministic SQL to stdout or output file
modes:
  check:
    - connect read-only where supported
    - fail when required session schema versions are missing or incompatible
    - make no schema changes
  apply:
    - resolve session.rdb.source and the effective DSN
    - acquire the plugin-defined migration lock where supported
    - apply pending versioned statements transactionally where supported
    - record the resulting plugin schema version
rules:
  - require selected and imported sessionstore/sqlite
  - reject Redis backend because it has no SQL schema
  - never inspect or migrate application-owned tables
  - refuse downgrade and destructive migration without a future explicit contract
  - redact credentials and sensitive DSN parameters from output and errors
  - nonzero exit reports connection, validation, lock, or migration failure
ownership:
  provider: api:session-backend-plugin schema provider
  lifecycle: command closes only connections it opens
```
