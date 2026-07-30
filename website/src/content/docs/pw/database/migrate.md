---
title: pw migrate
description: Inspect, apply, and roll back schema migrations.
sidebar:
  order: 1
---

```sh
pw migrate <action> [--dir <path>] [--dsn <dsn>] [--yes] [--dry-run]
```

## Actions

| Action | Effect |
| --- | --- |
| `status` | applied and pending migrations |
| `version` | current schema version |
| `up` | apply all pending migrations |
| `up-by-one` | apply the next pending migration |
| `up-to <version>` | apply through the given version |
| `down` | roll back the most recent migration |
| `down-to <version>` | roll back to the given version |
| `create <name>` | write the next migration file |
| `validate` | check the migration set |
| `snapshot` | capture the current schema |

`up-to` and `down-to` require a version argument; `create` requires a name.
Every other action rejects positional arguments rather than ignoring them.

## Options

| Option | Effect |
| --- | --- |
| `--dir <path>` | migration directory, overriding `migration.dir` |
| `--dsn <dsn>` | target database, overriding the application's configuration |
| `--yes` | confirm a destructive action without prompting |
| `--dry-run` | report what would happen without changing anything |

## Migration files

Files live in `migration.dir` — `migrations/` by default — and use goose's
format:

```sql
-- +goose Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE users;
```

```sh
pw migrate create add_email
```

writes the next numbered file and prints its path relative to the project root.

## Where the DSN comes from

Without `--dsn`, `pw` does not recreate the application's configuration rules.
It asks **the application** for the resolved DSN. The migration therefore
targets the same database the server would open: the TOML file selected by
`APP_ENV`, followed by environment and flag overrides.

The resolved DSN returns over a pipe, never as a process argument visible in a
process list. Any error printed by `pw` redacts it as well.

This is also why most migrate actions need a project: without `--dir` and
`--dsn` there is nothing to ask.

## During development

[`pw dev`](/pw/project/dev/) applies pending migrations at startup and whenever
the migration directory changes. Direct migration commands are therefore most
useful for inspection, rollback, and deployment. To take full control, disable
the automatic behavior:

```toml
[migration]
auto = false
```

## See also

- [Database Migrations](/productivity/migrations/) — the file format and the workflow around this command.
- [Queries](/guides/backend/queries/) — writing the SQL.
- [`pw seed`](/pw/database/seed/) — loading data after the schema is in place.
