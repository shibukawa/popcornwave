---
title: Queries
description: Typed .pw.sql statements, conditional SQL, and transactions over the configured connections.
sidebar:
  order: 1
---

SQL remains visible as SQL, but its boundary with Go becomes typed. You write
statements in `.pw.sql` files; `pw generate` compiles them into functions that
take a `context.Context` and return declared result types.

## Code generation

No SQL in a `.pw.sql` file is parsed at request time. `pw generate` compiles each
file into a `_pw_gen.go` beside it, and what the application calls is the
generated function. That file is build output: Git ignores it, and regenerating
recreates it.

Three commands run it. `pw dev` watches the project's sources and regenerates
whenever one changes, then rebuilds and restarts. `pw build` generates before it
compiles, and [`pw prepare`](/pw/project/prepare/) is that same work stopping
short of the compiler, for a build that TinyGo or your own `go build` drives.
`pw generate` runs it once by hand.

The scan is not the whole module. `popcornwave.toml` names directories per
purpose, and `.pw.sql` belongs to the `queries` purpose:

```toml
[generate]
queries = ["queries"]
```

The directory is walked recursively. A `.pw.sql` outside it is reported and
skipped rather than failing the run, so a fixture can sit beside your code:

```
pw: samples/report.pw.sql is outside generate.queries and is not generated from; list its directory to include it
```

A project with no SQL at all still declares the key as `queries = []`. The empty
list is a decision the next reader can see; a missing key is an error.
[`pw generate`](/pw/project/generate/) lists every purpose.

## A statement

```sql
package queries

type AccessCounter {
  count: int
}

export statement IncrementAccess(): sql.one<AccessCounter> {
INSERT INTO access_counter (id, count)
VALUES (1, 1)
ON CONFLICT(id) DO UPDATE SET count = access_counter.count + 1
RETURNING count
}
```

- `type` declares the result shape.
- `export statement` declares the function name, its typed parameters, and its
  result kind.
- `{name}` inside the SQL body binds a declared parameter.

```go
counter, err := queries.IncrementAccess(r.Context())
```

The context carries more than cancellation. It contains the pool in an ordinary
request and the active transaction inside `pw.Transaction`, which is why the
same generated function works in both places.

## Types

| Template type | Go type |
| --- | --- |
| `string`, `decimal` | `string` |
| `bool` | `bool` |
| `int` | `int` |
| `float` | `float64` |
| `bytes` | `[]byte` |
| `datetime`, `date`, `time` | `time.Time` |
| `url` | `url.URL` |

`T[]` is a slice and `T?` is optional.

## Statement kinds

| Kind | Returns |
| --- | --- |
| `sql.exec` | `sql.Result` — for INSERT, UPDATE, DELETE |
| `sql.one<T>` | `T`; zero rows is `sql.ErrNoRows`, several rows is an error |
| `sql.optional<T>` | `*T`; zero rows is `nil, nil` |
| `sql.many<T>` | `iter.Seq2[T, error]`, streamed rather than accumulated |
| `sql.predicate` | a private reusable condition, no public function |
| `sql.relation<T>` | a private typed subquery, no public function |

`sql.many` returning an iterator matters for large result sets — rows are not
collected into a slice first:

```go
for user, err := range queries.ListUsers(ctx) {
	if err != nil {
		return err
	}
	// ...
}
```

## Parameters

Every `{name}` becomes a prepared-statement placeholder. Template expressions
are never concatenated into SQL text, and handwritten placeholders are
rejected. Value binding therefore cannot create an injection-prone query.

```sql
export statement FindUser(id: int): sql.one<User> {
SELECT id, name FROM users WHERE id = {id}
}
```

That guarantee depends on a strict boundary: parameters bind **values**, not SQL
structure. They cannot substitute table names, column names, operators, or sort
directions.

The placeholder syntax the generator emits — `$1` for PostgreSQL, `?` for MySQL
and SQLite — comes from `project.database` in `popcornwave.toml`. You write
`{name}` either way; only the compiled output differs. See
[Choosing the database](/pw/project/init/#choosing-the-database).

### Slice expansion

A slice parameter expands into an `IN` list:

```sql
export statement FindUsers(ids: int[]): sql.many<User> {
SELECT id, name, active
FROM users
WHERE id IN ({ids})
ORDER BY id
}
```

An empty slice is a builder error. Handle the empty case in the caller, or use
conditional SQL to restructure the query.

## Conditional SQL

```sql
export statement SearchUsers(name: string, onlyActive: bool): sql.many<User> {
SELECT id, name, active
FROM users
WHERE name LIKE {name}
{if onlyActive}
  AND active = TRUE
{/if}
ORDER BY id
}
```

`{else}` is available too. Conditions must be `bool`. Only the branches that are
included consume placeholders, so numbering stays aligned.

### The connective is yours

A false branch drops its text and nothing else. Notice where the `AND` sits in
that example: inside the block, so it leaves with the predicate it joins. Put it
outside — or make the *first* predicate conditional — and a false condition
leaves the connective behind:

```sql
WHERE
{if byTitle}
  title = {title}
{/if}
{if onlyDone}
  AND done = TRUE
{/if}
```

With `byTitle` false, that builds `WHERE AND done = TRUE`. With both false, it
builds a `WHERE` followed by `ORDER BY`. Neither is caught: generation succeeds,
the build succeeds, and the statement is sent as written, so what you get is the
database's syntax error at request time. There is no pass that trims a leading
`AND`, and adding one would mean guessing which operator a half-written
condition wanted.

The habit that avoids it is the one the first example uses: **write each
connective inside the block that owns its predicate.** That leaves the question
of the first predicate, and there are two answers. If one condition is always
present — the owning account, the tenant, a soft-delete flag — put it first
unconditionally and let every optional predicate carry its own `AND`. If every
predicate is genuinely optional, anchor the clause instead:

```sql
WHERE 1 = 1
{if byTitle}
  AND title = {title}
{/if}
{if onlyDone}
  AND done = TRUE
{/if}
```

`1 = 1` costs nothing at the planner and makes every branch uniform, which is
also what makes the statement readable when a third condition arrives.

UPDATE and DELETE are the exception, and they are stricter. A mutation whose
WHERE could empty out is refused while generating, because the failure there is
not a syntax error but a full-table write:

```
queries/todos.pw.sql:41:1: UPDATE and DELETE statements require a WHERE clause that is non-empty on every branch
```

So the rule is worth remembering by its asymmetry: on a SELECT a dangling
connective reaches the database, and on a mutation it never reaches the compiler.

### The result shape cannot vary

Conditional SELECT or RETURNING columns are rejected because no single generated
type could describe every branch.

## Predicates and relations

A `sql.predicate` is a reusable WHERE fragment:

```sql
statement MinimumID(id: int): sql.predicate {
  id >= {id}
}

export statement FindRecentUsers(minimum: int): sql.many<User> {
SELECT id, name, active
FROM users
WHERE {MinimumID(minimum)}
ORDER BY id
}
```

A `sql.relation<T>` is a typed subquery usable in FROM or JOIN:

```sql
statement ActiveUsers(minimumID: int): sql.relation<ActiveUser> {
SELECT id, name
FROM users
WHERE id >= {minimumID} AND active = TRUE
}

export statement ListActiveUsers(minimumID: int, name: string): sql.many<ActiveUser> {
SELECT active_users.id, active_users.name
FROM subquery ActiveUsers(minimumID) AS active_users
WHERE active_users.name = {name}
ORDER BY active_users.id
}
```

Subquery and outer arguments share one placeholder sequence in final SQL order.
Aliases are explicit, lowercase, and snake_case. Recursive relations are
rejected.

## Two safety rules

**UPDATE and DELETE require a WHERE clause.** Generation fails if one is missing
outright, and if the WHERE is conditional and could disappear at run time, the
builder refuses to execute. There is no opt-in for a deliberate full-table
modification — write it as a migration.

**SELECT columns must match the result type**, in order and by name or alias.
Combined with the rule against conditional SELECT columns, this keeps the
generated struct an accurate description of every row the statement can return.

## Transactions

```go
err := pw.Transaction(r.Context(), func(ctx context.Context) error {
	if _, err := queries.InsertUser(ctx, name); err != nil {
		return err
	}
	return queries.RecordAudit(ctx, "user.created")
})
```

The transaction boundary remains explicit, and no request is wrapped in one
automatically. Frameworks that open a transaction when the request starts and
commit when it ends make the common case pay for the rare one: a page that reads
one row, or a handler that writes exactly one, buys a `BEGIN` and a `COMMIT` it
had no use for, plus a connection held for the whole request rather than for the
statement. Here that cost is charged only where the boundary is asked for.

The other half of the reason outlives the benchmark. A transaction is where a
database exposes what it is actually good at — isolation levels, a read-only
transaction that a replica can serve, savepoints, the choice of committing
before a slow call rather than after it. A layer that opens and closes the
boundary for you has to pick one behaviour for all of that, and what it picks is
the conservative default. Leaving the boundary in the application keeps those
choices reachable.

Nesting still works. An inner `pw.Transaction` opens a savepoint, so its failure
rolls back only the inner work while the outer transaction remains usable. A
driver without known savepoint support returns `ErrSavepointUnsupported` instead
of silently flattening the nesting.

Raw access is there when a query does not fit the generated layer:

```go
db, ok := pw.DB(r.Context())
```

On SQLite and MySQL that hands back the pool itself. On PostgreSQL `ok` is
`false`: requests run on a native pgx pool with no `*sql.DB` behind them, which
is what removes the `database/sql` locks from the query path. Generated
statements and `pw.Transaction` behave identically on every engine — reach for
them first, and see [Interoperability](/appendix/interoperability/) when a
third-party library needs a handle of its own.

## Which connection ran it

Nothing above names a database. A statement that says nothing about where it
runs goes to the default connection group; a write, or a whole transaction
against a reader-writer cluster, pins one with `pw.SelectDB`. That lives with
the connections themselves, in
[Relational databases](/guides/storage/rdb/), along with the `[middleware.rdb]`
section, the DSN schemes, and the import each engine needs.

A generated function never learns the topology, which is why it can be silent
about it. One development SQLite file answers every group name, so the code
above runs unchanged against a cluster and against nothing but that file.

## Seeing what ran

In `dev`, every generated statement is logged with its duration, and anything
slower than a threshold brings its query plan and a paste-able rerun snippet
with it — without a line of change in your code:

```
level=WARN msg="sql executed" sql="SELECT name FROM items WHERE name = $1"
  duration=240ms operation=query driver=sqlite outcome=ok args=[alpha] slow=true
  explain="id=2 parent=0 detail=SCAN items"
  reproduction=".parameter set $1 'alpha'\nSELECT name FROM items WHERE name = $1;"
```

See [Slow Query Diagnostics](/productivity/query-diagnostics/).

The complete language — every statement kind, the generated signatures, the
`export` casing rule, and `ScanRows` for grouping JOIN rows — is
[SQL Query Format](/reference/sql-templates/).

Schema and starting rows are a separate concern from the statements above, and
they live with the rest of the development tooling: [Database
Migrations](/productivity/migrations/) and [Seed
Data](/productivity/seed-data/).

