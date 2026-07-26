---
title: pw seed
description: Load seed datasets into the configured database.
sidebar:
  order: 2
---

```sh
pw seed [--dir=testdata/seed] [name...]
```

Applies seed datasets to the database the application is configured to use.

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

Several tables can appear in one file. Order matters when rows reference each
other, both within a file and across the names you pass.

## Shared with tests

These are the same files the test helpers load through `testutil.WithSeed`, so
a fixture cannot drift between the CLI and the test suite:

```go
server := testutil.TestRun(t, Handlers(), nil,
	testutil.WithMigrations("../migrations"),
	testutil.WithSeed("initial"),
)
```

See [Testing](/guides/testing/).

## Where the DSN comes from

As with [`pw migrate`](/pw/database/migrate/), `pw` asks the application for its
DSN instead of reimplementing configuration precedence, and redacts it from any
error it reports. The SQL dialect is derived from that DSN.

Seeding runs against whatever `APP_ENV` selects, so check which environment you
are in before running it against something that is not a development database.
