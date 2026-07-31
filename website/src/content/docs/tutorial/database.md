---
title: 3. Storing the memos
description: Write a migration, compile SQL into typed functions, and replace the in-memory list with a table.
sidebar:
  order: 3
---

Restart `pw dev` and every memo from chapter 2 is gone. The list lives in a
slice, and the slice lives in a process.

Moving it into the database means three new pieces: a migration that creates the
table, a `.pw.sql` file that compiles into typed Go functions, and a handler
that calls them. The project already has everything needed to run all three —
`pw init` scaffolded the database when you created the project, which is why
`config.dev.toml` already carries a DSN. Twenty minutes or so.

:::note[Where this starts]
From chapter 2: `handlers/home.pw.html` with `Memo`, `handlers/memos.go` holding
the list in memory, and `handlers/home_handler.go` with `GET /{$}` and
`POST /memos`.
:::

## 1. A migration for the table

Migrations are plain SQL files in [goose](https://github.com/pressly/goose)
format, numbered in the order they must be applied. Create
`migrations/00002_create_memos.sql`:

```sql
-- +goose Up
CREATE TABLE memos (
    id INTEGER PRIMARY KEY,
    body TEXT NOT NULL
);

-- +goose Down
DROP TABLE memos;
```

The two annotations divide the file: `Up` is what this version does, `Down` is
what undoes it. Writing the `Down` half now is cheap; reconstructing it later,
from a schema three versions further along, is not.

`00001_init.sql` was already there — the scaffold's example, creating a `users`
table this tutorial never reads. Leave it. It is applied, it costs nothing, and
renumbering an applied migration is the one thing migrations must never do.

`pw dev` applies pending migrations on startup and again whenever a file in the
migration directory changes, so saving this file is enough:

```
up	2	00002_create_memos.sql	1ms
version 1 -> 2
```

Outside the loop, `pw migrate up` does the same thing, and `pw migrate status`
answers what has been applied. See
[Migrations](/productivity/migrations/).

## 2. SQL that compiles

Create `queries/memos.pw.sql`:

```sql
package queries

type Memo {
  id: int
  body: string
}

export statement ListMemos(): sql.many<Memo> {
SELECT id, body FROM memos ORDER BY id DESC
}

export statement CreateMemo(body: string): sql.exec {
INSERT INTO memos (body) VALUES ({body})
}
```

The shape is the same one `.pw.html` uses: a package line, a declared result
type, and exported declarations with typed parameters. `pw generate` turns each
`export statement` into a Go function beside the source.

The result kind decides the signature. `sql.many<Memo>` returns
`iter.Seq2[Memo, error]` — rows are streamed rather than collected into a slice,
which is what keeps a large table from becoming a large allocation. `sql.exec`
returns a `sql.Result`, which is what an INSERT has to offer.

`{body}` is a parameter, and it becomes a prepared-statement placeholder — `?`
here, `$1` on PostgreSQL, decided by `project.database` in `popcornwave.toml`.
The generator will not concatenate a template expression into SQL text and
rejects a handwritten placeholder, so a statement written this way cannot become
an injection. The boundary is exactly that: parameters bind **values**, never
table names, column names, or sort directions.

Two rules will stop generation later, and both are worth knowing before you meet
them. An UPDATE or DELETE without a WHERE clause is rejected outright. And the
SELECT columns must match the declared result type, in order and by name — which
is what makes `Memo` an accurate description of every row the statement can
return. [Queries](/guides/backend/queries/) covers conditional SQL, slice
expansion, and reusable predicates.

## 3. The handler talks to the table

```go
// handlers/home_handler.go
package handlers

import (
	"context"
	"net/http"

	"memoapp/queries"

	"github.com/shibukawa/popcornwave/pw"
	httpbind "github.com/shibukawa/tinybind-go"
)

func init() {
	mux.HandleFunc("GET /{$}", home)
	mux.HandleFunc("POST /memos", createMemo)
}

func home(w http.ResponseWriter, r *http.Request) {
	list, err := listMemos(r.Context())
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	pw.WriteHTML(w, r, Home(HomeParams{Memos: list}))
}

type createMemoInput struct {
	Body string `payload:"body" check:"required,maxlen=200"`
}

func createMemo(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[createMemoInput](r)
	if err != nil {
		mapped, fieldError := httpbind.AsHTTPError(err)
		if !fieldError || len(mapped.Fields) == 0 {
			pw.WriteProblem(w, r, pw.BadRequest(err))
			return
		}
		list, listErr := listMemos(r.Context())
		if listErr != nil {
			pw.WriteProblem(w, r, listErr)
			return
		}
		pw.WriteHTML(w, r, Home(HomeParams{
			Memos: list,
			Draft: r.PostFormValue("body"),
			Error: mapped.Fields[0].Message,
		}))
		return
	}
	if _, err := queries.CreateMemo(r.Context(), input.Body); err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// listMemos turns the streamed rows into the slice the template renders.
func listMemos(ctx context.Context) ([]Memo, error) {
	var list []Memo
	for row, err := range queries.ListMemos(ctx) {
		if err != nil {
			return nil, err
		}
		list = append(list, Memo{Id: row.Id, Body: row.Body})
	}
	return list, nil
}
```

Then delete `handlers/memos.go`. The slice is gone.

Two details in that file are worth pausing on.

**The context carries the connection.** No handle is passed to
`queries.CreateMemo`; it takes a `context.Context` and finds the pool there. The
same call inside `pw.Transaction` finds the active transaction instead, which is
why one generated function works in both places without a variant that takes a
`*sql.Tx`.

**There are now two `Memo` types**, and `listMemos` converts between them.
`queries.Memo` describes a row; `handlers.Memo` describes what the page renders.
Merging them would be less code today and a worse boundary tomorrow, because the
first column a page stops showing — or the first field it needs that no column
supplies — would have to be resolved in whichever type was carrying both jobs.

## 4. Run it

Save everything. `pw dev` regenerates `queries/memos_pw_gen.go`, rebuilds, and
restarts. Add a memo, then stop `pw dev` with `Ctrl-C` and start it again: the
memo is still there.

In `dev`, every generated statement is logged as it runs:

```json
{
  "level": "INFO", "msg": "sql executed",
  "sql": "\nINSERT INTO memos (body) VALUES (?)\n",
  "duration": 0.601708, "operation": "exec",
  "driver": "sqlite", "rows_affected": 1, "outcome": "ok", "args": "eggs"
}
```

A statement slower than the configured threshold brings its query plan and a
paste-able rerun snippet with it, without a line of change in your code — see
[Query Diagnostics](/productivity/query-diagnostics/).

The database itself is the file `memoapp.db`, named by the DSN in
`config.dev.toml`. Deleting it and letting `pw dev` re-apply the migrations is a
perfectly good reset while the schema is still moving.

## What you have now

- A schema under version control, applied by the same loop that builds the code.
- SQL that stays SQL, compiled into functions whose parameters and rows are
  typed.
- A page whose contents survive a restart.

Every visitor still sees every memo. Chapter 4 gives the application a notion of
who is asking.

- [4. Signing in](/tutorial/login/) — the next chapter.
- [Queries](/guides/backend/queries/) — conditional SQL, transactions, read replicas.
- [Migrations](/productivity/migrations/) and [Seed data](/productivity/seed-data/) — the rest of the schema workflow.
