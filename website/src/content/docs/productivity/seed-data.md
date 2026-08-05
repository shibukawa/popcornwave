---
title: Seed Data
description: What seed data is, how a dataset file is written, and why the CLI and the test suite read the same files.
sidebar:
  order: 8
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

### If you know DBUnit

The model is DBUnit's: one file describes a set of tables, and the same file
serves both as the state you load and as the state you compare against. The keys
that shape either direction come from that lineage and work as described under
[Fixtures](/productivity/testing/#fixtures) — `_operation` for the per-table
operations DBUnit spells `CLEAN_INSERT`, `INSERT`, `TRUNCATE_TABLE`, and
`DELETE`; `_match` for whether unlisted rows are tolerated; matcher values like
`[notnull]` or `[currentdate, 2m]` for columns whose value cannot be written
down in advance.

What does not carry over is the file format. Datasets are YAML only. A DBUnit
XML dataset is not converted and not read: `Resolve` accepts the name because it
has an extension, and the YAML parser then rejects the first line with a message
about a string where a mapping was expected. Nothing in Popcorn Wave or in
[dbtestify](https://github.com/shibukawa/dbtestify), the library underneath,
parses XML, CSV, or Excel — porting an existing DBUnit suite means converting
the datasets.

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
