---
title: pw seed
description: Load seed datasets into the configured database.
sidebar:
  order: 2
---

```sh
pw seed [--dir=testdata/seed] [name...]
```

For what seed data is, and where it stops being a
[migration](/productivity/migrations/), see [Seed Data](/productivity/seed-data/).

Seed data is useful only when it lands in the intended database. `pw seed`
applies datasets to the database resolved from the application's own
configuration.

## Options and arguments

| Argument | Effect |
| --- | --- |
| *(none)* | apply every dataset in the seed directory |
| `name...` | apply only the named datasets, in the order given |
| `--dir <path>` | seed directory, defaulting to `testdata/seed` |

Names are paths relative to the seed directory and the `.yaml` extension may be
omitted, so `pw seed users` and `pw seed users.yaml` are the same request.

Each file is reported as it is applied:

```
seeding testdata/seed/users.yaml
seeding testdata/seed/orders.yaml
```

## Dataset format

A dataset is a table name mapped to a list of rows:

```yaml
member:
- { id: 1, name: Frank }
- { id: 2, name: Grace }
- { id: 3, name: Heidi }
```

One file may contain several tables. When rows reference one another, order
matters both inside the file and across the dataset names passed to the command.

Each table is emptied before its rows are inserted. An `_operation` block
selects something else per table — `insert` to add without clearing, `upsert`,
`truncate`, or `delete` — and is documented with the rest of the dataset format
under [Fixtures](/productivity/testing/#adding-to-a-table-instead-of-replacing-it).

## Shared with tests

The test helpers load these exact files through `testutil.WithSeed`. The CLI and
test suite therefore share one fixture instead of maintaining two versions:

```go
server := testutil.TestRun(t, Handlers(), nil,
	testutil.WithMigrations("../migrations"),
	testutil.WithSeed("initial"),
)
```

See [Testing](/productivity/testing/).

## Where the DSN comes from

As with [`pw migrate`](/pw/database/migrate/), `pw` asks the application for its
resolved DSN instead of duplicating configuration precedence. Errors redact the
value, while the DSN still determines the SQL dialect.

That consistency also raises the risk: seeding follows whatever `APP_ENV`
selects. Confirm the environment before targeting any database that is not for
development.
