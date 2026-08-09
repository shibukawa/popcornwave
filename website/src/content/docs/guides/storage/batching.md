---
title: Batching
description: Cutting the cost of many statements — a transaction on SQLite, a pipelined pgx batch on PostgreSQL, and what MySQL charges in return.
sidebar:
  order: 2
---

An import that inserts five hundred rows runs five hundred generated
statements, and each one waits for the previous reply before it leaves. The
statements are individually fast and the loop is not, because what it spends is
not query time.

Where that cost actually goes decides the fix, and it is not the same on every
engine. Start with a transaction. On SQLite that is the whole answer, and on
PostgreSQL it is the thing you need before batching means anything.

## First, a transaction

Every statement outside a transaction is its own transaction, which SQLite pays
for with an fsync each time. Wrapping the loop removes all but one:

```go
err := pw.Transaction(r.Context(), func(ctx context.Context) error {
	for _, name := range names {
		if _, err := queries.InsertItem(ctx, name); err != nil {
			return err
		}
	}
	return nil
})
```

The driver measures two hundred inserts on a SQLite file at roughly fifty
milliseconds without the transaction and about one with it, on all of its
backends. Nothing below improves on that, because there is no network to
amortise — a SQLite batch is a transaction with a queue in front of it. If your
database is SQLite, this page is finished.

On a server engine the transaction is still worth opening, and it is no longer
sufficient. Five hundred round trips remain five hundred round trips.

## PostgreSQL: one round trip

pgx can send a whole set of statements before reading any reply, and the server
runs them as one implicit transaction. That needs the pgx connection, and
`pw.DB` does not have one to give — requests run on a native pgx pool with no
`*sql.DB` behind them. `postgres.WithConn` is the way to it:

```go
package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/database/postgres"
	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/tinygodriver/database/pgx"
)

func ImportItems(w http.ResponseWriter, r *http.Request, names []string) {
	ctx, span := pw.StartSpanKind(r.Context(), "import-items", pw.SpanKindClient)
	defer span.End()

	err := postgres.WithConn(ctx, func(conn *pgx.Conn) error {
		batch := &pgx.Batch{}
		for _, name := range names {
			batch.Queue("INSERT INTO items (name) VALUES ($1)", name)
		}
		results := conn.SendBatch(ctx, batch)
		for range names {
			if _, err := results.Exec(); err != nil {
				_ = results.Close()
				return err
			}
		}
		return results.Close()
	})
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	pw.Logger(ctx).Info("imported", pw.Int("rows", len(names)))
}
```

Queued reads work the same way, through `results.Query` and `results.QueryRow`.
Prepared statements are unaffected: pgx caches and reuses them inside a batch
exactly as it does outside one.

Call this inside `pw.Transaction` and the callback receives the connection that
transaction is already executing on, so the batch joins it and rolls back with
it. Call it outside one and a pooled connection is leased for the call and
returned afterwards. Either way, nothing derived from the connection may
outlive the callback — read your rows and close your results before returning,
because the connection goes back to the pool the moment you do.

A group that is not PostgreSQL returns an error naming the engine it found
instead, so a handler written against pgx fails loudly on a SQLite deployment
rather than subtly.

### Nothing here reaches the query log

Generated statements are logged with their duration, their plan, and a
paste-able rerun. Work done through `WithConn` is not: it bypasses the executor
those diagnostics attach to, and instrumenting it would mean wrapping every pgx
call the callback might make.

Write the record yourself. `pw.Logger` reads the span active on the context, so
a log line inside the span above already correlates with it — that is the whole
wiring, and it is why the example opens a span before it opens the batch. Log
what a batch makes hard to recover afterwards: how many statements went, and
how long the exchange took. Per-statement timing is what you gave up.

## MySQL: possible, and a trade

`pw.DB` does return a pool on MySQL, so the driver's own batch package is
reachable with no framework help:

```go
db, ok := pw.DB(ctx)
if !ok {
	return errors.New("no pool on this connection")
}
batch := &sqlbatch.Batch{}
for _, name := range names {
	batch.Queue("INSERT INTO items (name) VALUES (?)", name)
}
return sqlbatch.Send(ctx, db, batch)
```

Read what it costs before you reach for it. MySQL has no pipelining, so the
package joins the statements into one multi-statement command, and that needs
two DSN settings the framework deliberately does not set for you:
`multiStatements=true` and, whenever a statement carries arguments,
`interpolateParams=true`. The first widens what an injection reaching your SQL
text can do, on every connection in the deployment. The second is what makes
the batch possible at all — a multi-statement command cannot be prepared, so
the driver renders your arguments into the SQL itself instead of binding them
server-side.

You also give up detail. Only writes can be batched, the server reports one
error for the whole command so the failing statement is usually unidentifiable,
and the batch cannot join a transaction you already opened.

Take it for a write-heavy import whose operator has agreed to both DSN
settings. Otherwise stay with a transaction, which costs nothing and is
available everywhere.

## When not to batch

If the statements are inserts into one table, `INSERT INTO items (name) VALUES
($1), ($2), ($3)` beats every option on this page. It parses once instead of N
times, it is one round trip on every engine including MySQL, it needs no
escape hatch and no DSN change, and `.pw.sql` slice expansion writes it for
you. Reach for a batch when the statements genuinely differ.

Two more cases rule it out. Inside a PostgreSQL batch the server may parse
every queued statement before running any of them, so a `CREATE TABLE`
followed by an insert into that table fails where the same pair inside
`pw.Transaction` succeeds — keep DDL out of batches. And an implicit
transaction is still a transaction, so `VACUUM`, `CREATE DATABASE`, and
`CREATE INDEX CONCURRENTLY` are rejected in any batch of more than one
statement.

One promise a batch does not make: reads inside it do not necessarily share a
snapshot. PostgreSQL takes a fresh one per statement at `READ COMMITTED`, while
SQLite and MySQL share one across the surrounding transaction. A batch promises
order and atomicity. Consistency of view is the transaction's job, and its
isolation level cannot be set on the implicit one a bare batch runs in — queue
the batch inside `pw.Transaction` when you need a stronger level.

`WithConn` serves more than batches. `CopyFrom` for bulk ingest, `LISTEN` and
`NOTIFY`, and `errors.As` against `*pgx.PgError` for SQLSTATE all live behind
the same callback. See
[Interoperability](/appendix/interoperability/) for the rest of what a
PostgreSQL deployment can reach, and
[Queries](/guides/storage/queries/) for the generated layer these examples step
outside of.
