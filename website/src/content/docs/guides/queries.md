---
title: Queries and migrations
description: Typed .pw.sql statements, conditional SQL, transactions, goose migrations, and seed data.
sidebar:
  order: 4
---

SQL is written in its own source file and compiled into Go by `pw generate`.
The generated functions take a `context.Context` and return typed results.

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

The context is not decoration: it carries the pool, and inside
`pw.Transaction` it carries the transaction. That is why the same generated
function works in both places.

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

Every `{name}` becomes a placeholder in the prepared statement. Template
expressions are never concatenated into SQL text, and handwritten placeholders
are rejected — so a query cannot be injection-prone by construction.

```sql
export statement FindUser(id: int): sql.one<User> {
SELECT id, name FROM users WHERE id = {id}
}
```

Parameters bind **values**, not structure. Table names, column names,
operators, and sort directions cannot be substituted.

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

The one hard restriction: **the result shape cannot vary.** Conditional SELECT
or RETURNING columns are rejected, because the generated type would no longer
describe every branch.

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

The boundary is always explicit — the framework never wraps a request in one for
you. Nesting is supported: an inner `pw.Transaction` opens a savepoint, so its
failure rolls back only its own work and the outer transaction stays usable. A
driver with no known savepoint support fails with `ErrSavepointUnsupported`
rather than silently flattening the nesting.

Raw access is there when a query does not fit the generated layer:

```go
db, ok := pw.DB(r.Context())
```

## Migrations

Migrations live in `migrations/` and use goose's format:

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
pw migrate status
pw migrate up
```

`pw dev` applies pending migrations on startup, so the everyday loop rarely
needs these directly. The full action list is in [pw migrate](/pw/database/migrate/).

## Database configuration

The pool lives under `[middleware.rdb]` and is **off by default**:

```toml
[middleware.rdb]
enabled = true
dsn = "sqlite://myapp.db"
connect_timeout = "5s"
max_open_conns = 1
max_idle_conns = 1
```

`dsn` is treated as a secret: redacted in configuration logs and in error
messages. See [Configuration](/guides/configuration/).

## Seed data

Seed files live in `testdata/seed/` and are shared by the CLI and the test
helpers, so a fixture cannot drift between them:

```yaml
member:
- { id: 1, name: Frank }
- { id: 2, name: Grace }
```

```sh
pw seed
```

See [pw seed](/pw/database/seed/) and [Testing](/guides/testing/).
