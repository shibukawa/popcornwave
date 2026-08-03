---
title: Queries
description: Typed .pw.sql statements, conditional SQL, transactions, and reader-writer connection groups.
sidebar:
  order: 2
---

SQL remains visible as SQL, but its boundary with Go becomes typed. You write
statements in `.pw.sql` files; `pw generate` compiles them into functions that
take a `context.Context` and return declared result types.

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

One restriction follows from typed results: **the result shape cannot vary.**
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

The transaction boundary remains explicit; the framework never wraps a request
in one automatically. Nesting still works. An inner `pw.Transaction` opens a
savepoint, so its failure rolls back only the inner work while the outer
transaction remains usable. A driver without known savepoint support returns
`ErrSavepointUnsupported` instead of silently flattening the nesting.

Raw access is there when a query does not fit the generated layer:

```go
db, ok := pw.DB(r.Context())
```

## Database configuration

The pool lives under `[middleware.rdb]` and is **off by default**. One
`[[middleware.rdb.connections]]` element is one pool, and one element is a
single database:

```toml
[middleware.rdb]
enabled = true

[[middleware.rdb.connections]]
group = "default"
dsn = "sqlite://myapp.db"
connect_timeout = "5s"
max_open_conns = 1
max_idle_conns = 1
```

`dsn` is treated as a secret, but only the credential is: the startup summary,
`pw doctor`, and a failure message all print
`postgres://*****@db.internal:5432/app` — scheme, host, port, and database name
kept, userinfo and query string dropped. Which database a process is attached to
is an operational fact, and a line that answers nothing is a line an operator
stops reading. A SQLite path or `:memory:` carries no credential and appears
whole. See [Configuration](/guides/architecture/configuration/).

The scheme selects the engine, and a server engine needs a blank import to
register it:

| Scheme | Engine | Import |
| --- | --- | --- |
| `sqlite://` | SQLite | already linked |
| `postgres://` | PostgreSQL | `_ "github.com/shibukawa/popcornwave/database/postgres"` |
| `mysql://` | MySQL, MariaDB | `_ "github.com/shibukawa/popcornwave/database/mysql"` |

`pw init` writes that import for you. Without it the pool refuses to open and
names the import to add rather than failing somewhere inside `database/sql`.
Keep the scheme in agreement with `project.database`: one decides which driver
runs the query, the other which syntax it was compiled to.

## Readers and writers

A reader-writer cluster is the same shape with more elements. Each names the
group it belongs to, and several elements may share one group — reads are spread
across them round robin. Because TOML reads every key after a `[[…]]` header as
part of that element, the plain `rdb` keys have to come first:

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

A connection element takes no CLI option and no environment variable of its own
— its identity is its position in the file — so `${NAME}` is how a per-connection
password stays out of the committed TOML. It is expanded while the file is read,
in string values only, and an undefined name fails the load rather than
expanding to nothing. Write `$$` for a literal `$`. Expanded or not, `dsn` stays
redacted in the startup summary and in errors.

Statements that say nothing about a group run on `default_group`. A write picks
its group explicitly:

```go
// One statement.
user, err := queries.CreateUser(pw.SelectDB(ctx, "writer"), name)

// A whole transaction — unpinned statements inside it stay on the writer.
err := pw.Transaction(ctx, func(ctx context.Context) error {
	return queries.RecordAudit(ctx, "user.created")
}, pw.OnGroup("writer"))
```

One transaction never spans two groups: a nested `pw.Transaction` naming a
different group returns `ErrCrossGroupTransaction` and leaves the outer one
usable. Inside a transaction you may still `SelectDB` a `readonly` group — that
read simply happens outside the transaction — but not a writable one: such a
write would look atomic without being atomic.

[Migrations](/productivity/migrations/), [seed data](/productivity/seed-data/),
and the session table go to `write_group`, or to the
narrower `migration_group` and `session.rdb.group` when they are set. A
`readonly` connection is never chosen for them, and configuring one there fails
at startup.

A configuration with a single connection — including the plain `dsn` form above
and every `testutil` run — answers *every* group name with that one database. So
code written for a cluster runs unchanged against one development SQLite file,
with no test-only branch.

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

See [Query Diagnostics](/productivity/query-diagnostics/).

Schema and starting rows are a separate concern from the statements above, and
they live with the rest of the development tooling: [Database
Migrations](/productivity/migrations/) and [Seed
Data](/productivity/seed-data/).

