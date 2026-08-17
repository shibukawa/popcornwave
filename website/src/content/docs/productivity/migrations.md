---
title: Database Migrations
description: The versioned record of schema change — file format, the development loop, deployment, and which database receives it.
sidebar:
  order: 8
---

One application has more than one database. There is the file on your laptop,
another on a colleague's, a throwaway one per CI run, staging, and production —
and they all have to end up with the same schema without anyone applying `ALTER
TABLE` by hand in five places.

A **migration** is one numbered, replayable step of schema change. Replaying the
same ordered set against any of those databases brings it to the same structure,
which is what makes the schema something the repository owns rather than
something each environment remembers on its own.

## Vocabulary

| Term | Meaning |
| --- | --- |
| migration | one file holding a forward change and its reversal |
| up / down | the forward direction and the rollback direction |
| version | the migration's number, which orders the set |
| applied / pending | already recorded in this database, or still waiting |

Applied versions are recorded **in the database itself**, by number. That
recording is what lets `pw migrate up` be safe to run twice: the second run finds
nothing pending.

## A migration file

Migrations live in `migrations/` and use goose's format — two annotated
sections in one `.sql` file:

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

Write the `Down` section even when you expect never to run it. A rollback you
cannot perform turns a bad deploy into an outage that lasts as long as writing
the reverse migration takes.

## The development loop

[`pw dev`](/pw/project/dev/) applies pending migrations at startup, so the usual
inner loop is: create the file, edit it, restart. You rarely type a migrate
command during development at all.

Deployment and rollback are the other half, and they are explicit:

```sh
pw migrate status
pw migrate up
pw migrate down
```

`status` before `up` is worth the extra second on any database you did not
create yourself. The full action list is in [pw migrate](/pw/database/migrate/).

## Which database receives them

Migrations go to `middleware.rdb.write_group`, or to the narrower
`middleware.rdb.migration_group` when it is set. A `readonly` connection is
never chosen, and configuring one as the migration target fails at startup
rather than at the first `ALTER TABLE`.

`pw` asks the application for its resolved DSN instead of reimplementing
configuration precedence — which means migrations follow whatever `APP_ENV`
selects. Confirm the environment before pointing the command at anything that is
not a development database. See [Relational databases](/guides/storage/rdb/) for
connection groups, and [Configuration Keys](/reference/configuration/) for the
keys themselves.

## In tests

`testutil.WithMigrations("../migrations")` applies the set before the test
server starts, and how the schema arrives depends on the engine:

- **SQLite** replays a cached snapshot into the copied database. That is what
  makes `sqlite://:memory:` work — an in-process database is unreachable by DSN,
  so SQL is transferred rather than a connection string.
- **PostgreSQL and MySQL** apply the migrations directly. A second `TestRun`
  against the same database applies nothing and reuses the schema, which is how
  a package of tests shares one prepared server.

Point a server DSN at a database dedicated to the test suite. Because applied
versions are recorded by number, a database already carrying another project's
version 1 makes your first migration look applied — and the schema never
arrives, with no error to read. See [Testing](/productivity/testing/).

A schema alone is rarely enough to run against. The rows that make it useful are
[Seed Data](/productivity/seed-data/).
