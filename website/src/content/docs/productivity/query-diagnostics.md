---
title: Query Diagnostics
description: Log every generated SQL statement, explain the slow ones, and get a snippet that reruns them by hand.
sidebar:
  order: 5
---

A slow page usually has one slow statement inside it, and finding that statement
is normally the tedious part: add a print, run it again, copy the SQL out of a
generated file, guess at the arguments, paste something into a shell that is
close to what ran but not quite it.

Query diagnostics does that work for you. Every function generated from a
`.pw.sql` file resolves its database handle through a single place in the
framework, so instrumenting that one place covers all of them. Nothing changes
in your code, and nothing changes in the generated file.

In `dev` it is already on.

## What ran

Each statement produces one record:

```
level=INFO msg="sql executed" sql="INSERT INTO items (name) VALUES ($1)"
  duration=412µs operation=exec driver=sqlite rows_affected=1 outcome=ok args=[alpha]
```

`duration` covers the executor call. For a `sql.many` statement that means the
query itself, not the loop where you scan its rows — those rows belong to your
code, so their cost is measured wherever you measure your handler.

A statement that fails still gets a record, with `outcome=error` and the message
beside the SQL that produced it. That pairing is often the whole diagnosis.

Inside a transaction, the record carries `tx_depth`, so a statement that ran in a
savepoint two levels down says so.

## Why it was slow

Once a statement passes `slow_threshold`, the record moves to `warn` and brings
two more fields with it:

```
level=WARN msg="sql executed" sql="SELECT name FROM items WHERE name = $1"
  duration=240ms operation=query driver=sqlite outcome=ok args=[alpha] slow=true
  explain="id=2 parent=0 detail=SCAN items"
  reproduction=".parameter set $1 'alpha'\nSELECT name FROM items WHERE name = $1;"
```

`explain` is the database's own plan, captured on the same connection and inside
the same transaction as the statement it describes — so it is the plan that
actually applied, not the plan a fresh session would have chosen.

It is plan-only. `EXPLAIN ANALYZE` would run your statement a second time, which
for anything that writes means running the write twice, so the framework never
reaches for it. The cost is one plan lookup on a statement that was already slow.

Each dialect gets its own form: `EXPLAIN QUERY PLAN` for SQLite, `EXPLAIN (FORMAT
JSON)` for PostgreSQL, `EXPLAIN FORMAT=JSON` for MySQL. A driver with no known
plan-only form says so once at startup and keeps the rest of the query log.

One case yields no plan at all, and it is worth recognising rather than
debugging: on SQLite, an `INSERT` with a `VALUES` list returns no plan rows —
including one carrying `ON CONFLICT` or `RETURNING`. `EXPLAIN QUERY PLAN`
describes how rows are *found*, and such a statement looks for none. The record
still arrives with its duration and its SQL; the `explain` field is simply
absent. `UPDATE`, `DELETE`, and `INSERT ... SELECT` do search for rows, so those
are explained normally.

## Rerunning it by hand

`reproduction` is the statement in a form you can paste into the database shell.
It binds the arguments rather than writing them into the SQL:

```
.parameter set $1 'alpha'
SELECT name FROM items WHERE name = $1;
```

That distinction is the whole point of the field. Substituting `'alpha'` into the
text gives you a statement that reads the same and can plan differently — the
value folds into a constant, an index looks cheaper or dearer than it did, and
you end up studying a query that is not the one that was slow. Binding keeps the
prepared shape intact, so the plan you reproduce is the plan you saw.

The snippet is dialect-specific: parameter directives for the `sqlite3` shell,
`PREPARE`/`EXECUTE` for `psql`, user variables plus a prepared statement for the
`mysql` client.

When a statement cannot be reproduced exactly, no snippet is emitted at all. That
happens when bind values are off, when a value was truncated by
`max_value_length`, when a value holds a character that would break the snippet,
or when the placeholder style does not match the dialect. A snippet that runs a
different query would be worse than no snippet, so the field simply stays absent.

## Settings

Everything lives under `[observability.query]`:

```toml
[observability.query]
enabled = "auto"          # auto is on in dev, off everywhere else
level = "info"
slow_threshold = "200ms"  # zero turns off explain and reproduction
slow_level = "warn"
bind_values = "auto"      # the only path by which row values reach a log
explain = true
reproduction = true
max_sql_length = 4096
max_value_length = 256
```

`auto` is what makes this a development aid rather than a switch you have to
remember. It resolves to on when `APP_ENV` is `dev` and off everywhere else.

Outside `dev`, both `enabled` and `bind_values` need an explicit `"on"`. They are
separate keys on purpose: turning on the query log in staging to time a statement
does not also start writing your users' data into the log. A non-development run
that enables either one says so at startup, because the person reading the logs
is not always the person who changed the configuration.

Setting `slow_threshold` to zero turns off slow detection, and with it both the
plan and the snippet, without touching their own switches.

## What is not covered

Only generated `.pw.sql` calls pass through the instrumented seam. Session, auth,
and migration statements go straight to the pool and produce no records — they
are framework plumbing rather than the application's own SQL, and keeping them
out is deliberate.

A record also carries no statement name. The seam sees the SQL text and its
arguments, which is what the executor is handed; the name of the `.pw.sql` block
it came from never reaches that layer.

## In tests

Test runs default to `dev`, so an application test that exercises generated
queries logs them too. That is usually what you want from a failing test. To
quiet it, set `enabled = "off"` in the configuration your tests load.

See [Queries and migrations](/guides/queries/) for the statements themselves and
[Configuration](/guides/configuration/) for how these keys are resolved.
