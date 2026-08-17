---
title: Seed Data
description: What seed data is, how a dataset file is written, and why the CLI and the test suite read the same files.
sidebar:
  order: 9
---

An empty schema runs, but it does not demonstrate anything. Open the application
against a freshly migrated database and every list is empty, every detail page is
a 404, and the first thing anyone does is type rows in by hand — differently on
each machine.

**Seed data** is those rows, written down. A file per dataset, in the
repository, applied by one command.

## Vocabulary

| Term | Meaning |
| --- | --- |
| seed data | rows loaded into a database to make it usable, separately from its schema |
| dataset | one YAML file: table names mapped to lists of rows |
| fixture | the same file when a test uses it as a known state — see [Fixtures](/productivity/testing/#fixtures) |

The line between a seed and a [migration](/productivity/migrations/) is worth
drawing precisely, because both put things into a database. A migration changes
**structure**, is versioned, is recorded as applied, and travels to every
environment including production. A seed inserts **rows**, carries no version,
is recorded nowhere, and is aimed at development and test databases. Running one
twice is not the same kind of event: a migration finds nothing pending, while a
seed inserts again.

## A dataset

```yaml
member:
- { id: 1, name: Frank }
- { id: 2, name: Grace }
- { id: 3, name: Heidi }
```

Files live in `testdata/seed/`. One file may contain several tables, and rows are
inserted in the order written — which matters as soon as one row references
another, both inside a file and across the files named on the command line.

```sh
pw seed                # every dataset in the directory
pw seed users orders   # only these, in this order
```

Names are relative to the seed directory and the `.yaml` extension may be
omitted, so `pw seed users` and `pw seed users.yaml` are one request. See
[pw seed](/pw/database/seed/).

## The dataset format

Datasets are YAML and nothing else. The model is DBUnit's and the key names come
from it, but no XML, CSV, or Excel dataset is read — neither Popcorn Wave nor
[dbtestify](https://github.com/shibukawa/dbtestify), the library underneath,
parses one.

### Tables, rows, and NULL

Every top-level key is a table name, except the ones beginning with an
underscore, which are directives. Each table holds a list of rows, and each row
maps column names to values.

A NULL is written `null`. What an *omitted* column means depends on whether its
neighbours mention it, because rows are inserted in batches and the column list
of one statement is the union of the keys its rows used:

```yaml
member:
- { id: 1, name: Frank, nickname: Frankie }
- { id: 2, name: Grace }          # nickname is inserted as NULL
```

A column no row in the table mentions is left out of the insert entirely, so the
schema default applies to it. That is how a `created_at` with a default, or a
serial `id`, stays out of a dataset. But a column one row supplies is part of
that statement for every row beside it, and the rows that left it out get NULL
rather than the default — which is a NOT NULL constraint violation if the column
has one, and a silently blank column if it does not.

A row may also carry a `_tag` list. Popcorn Wave parses it and never acts on it;
see [Row tags](/productivity/testing/#row-tags-are-parsed-but-never-filter).

### `_operation`: what happens to the table first

Each table is truncated and refilled unless the file says otherwise, so applying
a dataset twice leaves the same rows rather than twice as many.

```yaml
_operation:
  member: insert
  access_log: truncate

member:
- { id: 3, name: Heidi }
```

| Operation | Effect |
| --- | --- |
| `clear-insert` (default) | truncate the table, then insert the listed rows |
| `insert` | insert the listed rows, leaving what is already there |
| `upsert` | insert each listed row, updating it if the primary key exists |
| `truncate` | empty the table and insert nothing |
| `delete` | remove the rows whose primary keys the file lists |

Tables the file does not name are untouched by any of these.

`upsert` and `delete` are the two that need the table's primary keys, and
looking those up costs a second connection —
[a constraint worth reading before choosing one](/productivity/testing/#adding-to-a-table-instead-of-replacing-it).

The key is singular, and so is `_tag`. A plural is read as a table name instead,
so `_operations:` produces an error about a table that does not exist rather
than one about a misspelled directive.

### Values that only make sense as an expectation

The same file can be compared against a database instead of loaded into one, and
two parts of the format exist only for that direction.

A bracketed value is a matcher rather than a value:

| Value | Matches |
| --- | --- |
| `[null]` | a NULL, the same as writing `null` |
| `[notnull]` | anything other than NULL |
| `[any]` | any value |
| `[currentdate, 2m]` | a timestamp within the given duration of now, defaulting to `1m` |
| `[regexp, ^User .+ logged in$]` | a value whose text form matches the pattern |

```yaml
audit_log:
- { id: 1, created_at: [currentdate, 2m], message: [regexp, "^User .+ logged in$"] }
```

They cover the columns that cannot be written down in advance: a generated
timestamp, a message with an identifier embedded in it. Do not put one in a file
you seed with — a matcher reaches the driver as a value it cannot bind, and the
seed fails there.

`_match` is the other expectation-only key. It decides per table whether rows the
file does not list are tolerated, and which to choose is
[a judgement covered with the assertion side](/productivity/testing/#comparing-part-of-a-table).

## One file, two consumers

The test helpers read these exact files:

```go
server := testutil.TestRun(t, Handlers(), nil,
	testutil.WithMigrations("../migrations"),
	testutil.WithSeed("initial"),
)
```

`WithSeed` loads datasets after the schema is installed and before the server
starts; `WithSeedDir` moves the directory. Sharing the files is the point rather
than a convenience: a fixture maintained separately from the seed drifts, and it
drifts silently — the test keeps passing against a shape the development
database no longer has.

A browser suite is a third consumer of the same files: in the `pwdev` build the
application serves a seed endpoint, and a Playwright test reseeds the database
with one HTTP request between tests, as
[E2E Testing](/productivity/e2e-testing/) shows. One format, three readers, no
drift.

A test also reads these files in the other direction. `server.AssertDB(t,
"after_archive")` compares the database against a dataset and reports a per-table
diff, so the expected state after a request is another file beside this one
rather than a column of `SELECT` assertions. [Fixtures](/productivity/testing/#fixtures)
covers that round trip, along with the match strategies and the per-table
operations that make a dataset add to a table rather than replace it.

## Which database receives it

Seeds follow the same routing as migrations: `middleware.rdb.write_group`, or
`middleware.rdb.migration_group` when set, and never a `readonly` connection.

`pw seed` also resolves its DSN from the application's own configuration, which
means it obeys `APP_ENV` like everything else. That consistency is what makes
the command trustworthy, and also what makes it dangerous: check the environment
before seeding anything that is not a development database.
