---
title: Relational databases
description: The framework-owned connection set — engines, DSNs, pool bounds, and the reader-writer groups a statement is addressed to.
sidebar:
  order: 0
---

The application never opens a database. `[middleware.rdb]` describes the
connections, the framework opens them before the first request, and a generated
statement finds the right one through the request context. There is no
`SetDatabase` call to place correctly and no `*sql.DB` to thread through a
handler signature.

Configuration is therefore carrying the whole topology, which is what makes a
single SQLite file and a three-node reader-writer cluster the same application.
They differ in this file. They differ nowhere else.

## Turning it on

The pool is **off by default**. One `[[middleware.rdb.connections]]` element is
one pool, and one element is a single database:

```toml
[middleware.rdb]
enabled = true

[[middleware.rdb.connections]]
dsn = "sqlite://myapp.db"
connect_timeout = "5s"
max_open_conns = 1
max_idle_conns = 1
```

An element that names no `group` joins the group called `default`, so a project
with one database never writes a group name at all. `pw init --db=…` and
`pw add database` write this section for the engine you picked, together with
the blank import that engine needs.

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `false` | open the framework-owned pool |
| `default_group` | *(empty)* | group serving statements that pin none; required once there is more than one group |
| `write_group` | *(empty)* | group serving framework-owned writes; required once more than one group holds a writable connection |
| `migration_group` | *(empty)* | group receiving migrations and seeds; falls back to `write_group` |

Because TOML reads every key after a `[[…]]` header as part of that element,
these four have to come before the first connection.

## Engines

The DSN scheme selects the engine — not a `database/sql` driver name, because
one of the three registers none. Every engine reaches the binary through a
blank import, so an application carries no unused SQL driver:

| Scheme | Engine | Import |
| --- | --- | --- |
| `sqlite://` | SQLite | `_ "github.com/shibukawa/popcornwave/database/sqlite"` |
| `postgres://`, `postgresql://` | PostgreSQL | `_ "github.com/shibukawa/popcornwave/database/postgres"` |
| `mysql://` | MySQL, MariaDB | `_ "github.com/shibukawa/popcornwave/database/mysql"` |

`pw init` writes that import for you. Without it the pool refuses to open and
names the import to add, rather than failing somewhere inside `database/sql`
with a driver that was never registered. An unknown scheme fails the same way,
naming the schemes this build actually has.

What follows the scheme differs by engine, because each takes what its driver
takes: a path or `:memory:` for SQLite, a libpq URL for PostgreSQL, and a
go-sql-driver DSN for MySQL. MySQL gains `parseTime=true` unless the DSN sets
it, applied at the engine so no caller has to remember it.

The resolved engine also decides the dialect the rest of the framework reads —
savepoint support, `EXPLAIN` syntax, and the migration runner's dialect all
come from it. Keep the scheme in agreement with `project.database` in
`popcornwave.toml`: one decides which driver runs the query, the other which
syntax `pw generate` compiled it to.

PostgreSQL serves requests through a native pgx pool rather than through
`database/sql`, because the `sql.DB` layer costs a pool mutex and a per-call
mutex on every statement. Nothing about queries, transactions, or
configuration changes — the startup log names the path each connection took
(`path=native` against `path=database/sql`) — but two consequences are worth
knowing: `pw.DB` reports no `*sql.DB` on a PostgreSQL connection, and
`max_idle_conns` does not apply there, because a pgx pool prunes idle
connections by `conn_max_idle_time` rather than by count. Migrations and
seeding still run on `database/sql`; the bypass covers the request path only.

## One connection

| Key | Default | Meaning |
| --- | --- | --- |
| `group` | `"default"` | the name this connection is addressed by |
| `dsn` | *(empty)* | data source name; only its credential is masked where it is reported |
| `readonly` | `false` | never selected for a framework write |
| `connect_timeout` | `"5s"` | bounds the startup ping |
| `max_open_conns` | `0` | |
| `max_idle_conns` | `0` | |
| `conn_max_lifetime` | `"0s"` | |
| `conn_max_idle_time` | `"0s"` | |

`dsn` is treated as a secret, but only the credential is: the startup summary,
`pw doctor`, and a failure message all print
`postgres://*****@db.internal:5432/app` — scheme, host, port, and database name
kept, userinfo and query string dropped. Which database a process is attached
to is an operational fact, and a line that answers nothing is a line an
operator stops reading. A SQLite path or `:memory:` carries no credential and
appears whole.

A connection element takes no CLI option and no environment variable of its own
— its identity is its position in the file — so `${NAME}` is how a
per-connection password stays out of the committed TOML. It is expanded while
the file is read, in string values only, and an undefined name fails the load
rather than expanding to nothing. Write `$$` for a literal `$`. Expanded or
not, `dsn` stays redacted. See
[Application Configuration](/guides/architecture/configuration/).

## Readers and writers

A reader-writer cluster is the same shape with more elements. Each names the
group it belongs to, and several elements may share one group — reads are
spread across them round robin, though one request that reads twice from a
group stays on the connection it already has:

```toml
[middleware.rdb]
enabled = true
default_group = "replica"
write_group = "writer"

[[middleware.rdb.connections]]
group = "writer"
dsn = "postgres://app:${DB_PASSWORD}@writer.example/app"
max_open_conns = 20

[[middleware.rdb.connections]]
group = "replica"
dsn = "postgres://app:${DB_PASSWORD}@replica-1.example/app"
readonly = true

[[middleware.rdb.connections]]
group = "replica"
dsn = "postgres://app:${DB_PASSWORD}@replica-2.example/app"
readonly = true
```

Statements that say nothing about a group run on `default_group`. A write picks
its group explicitly:

```go
// One statement.
user, err := queries.CreateUser(pw.SelectDB(r, "writer"), name)

// A whole transaction — unpinned statements inside it stay on the writer.
err := pw.TransactionContext(pw.SelectDB(r, "writer"), func(ctx context.Context) error {
	return queries.RecordAudit(ctx, "user.created")
})
```

`pw.SelectDB` is the only thing that pins a group, for one statement and for a
whole transaction alike. There is no transaction-only spelling of it, so nothing
here can disagree about which group won.

One transaction never spans two groups: a nested `pw.Transaction` naming a
different group returns `ErrCrossGroupTransaction` and leaves the outer one
usable. Inside a transaction you may still `SelectDB` a `readonly` group — that
read simply happens outside the transaction — but not a writable one: such a
write would look atomic without being atomic.

[Migrations](/productivity/migrations/), [seed data](/productivity/seed-data/),
and the session table go to `write_group`, or to the narrower
`migration_group` and `session.rdb.group` when they are set. A `readonly`
connection is never chosen for them, and configuring one there fails at
startup.

A configuration with a single connection — including every `testutil` run —
answers *every* group name with that one database. So code written for a
cluster runs unchanged against one development SQLite file, with no test-only
branch.

A group looks like the place where failover would live, and it is not. There is
no health checking, no ejection, no failover between replicas, no replica-lag
awareness, and no read-your-writes routing. A replica that has fallen behind is
still selected. A read that must see its own write goes to the writer through
`SelectDB`, decided by the code that knows it matters — which is the only place
that does know.

## Startup and readiness

Every connection is opened and pinged within its own `connect_timeout` before
the first request is accepted. One that cannot answer stops the deployment and
is named by its label — `replica#2`, the group and its ordinal — so the message
says which of five replicas is unreachable. A set that was partly opened is
closed rather than served. What survives that is listed by the
[configuration summary](/productivity/startup-summary/), each connection with its
group, its `readonly` flag, its pool bounds, and its redacted DSN.

The [readiness endpoint](/guides/deployment/operational-endpoints/) pings every
connection for as long as the process runs. A replica that stops answering
makes the instance unready, because the default group is what the application
reads from, and an instance that cannot read is not ready no matter what the
writer says.

Shutdown closes every pool the framework opened, after the listener stops and
active handlers drain.

## Where the schema comes from

Nothing here creates a table. The connection set is opened against a schema
that already exists, so a first deployment fails on a missing table rather than
inventing one. Getting the schema there is
[Database Migrations](/productivity/migrations/), and starting rows are
[Seed Data](/productivity/seed-data/). Framework-owned tables — the session
table above all — ship their migration with the package that owns them and are
applied by the same run.

The statements that then run against these connections are
[Queries](/guides/storage/queries/).
