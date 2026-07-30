---
title: 4. Signing in
description: Add login sessions, protect the memo page, and give every account a list of its own.
sidebar:
  order: 4
---

One table, one list, and everyone who opens the page sees all of it. Giving each
person their own memos needs an answer to a question the application currently
cannot ask: who is this?

Login is the part of a framework that usually arrives as three routes, a
protocol library, and a week of reading. Popcorn Wave serves the routes itself.
What is left for the application is a column, a filter, and a decision about
which paths require a session — which is what this chapter is. Thirty minutes,
including the wizard.

:::note[Where this starts]
From chapter 3: memos in a table, `queries/memos.pw.sql` with `ListMemos` and
`CreateMemo`, and a handler that renders the list and accepts a POST.
:::

:::caution[Scope]
This chapter covers **sessions and login**: keeping a signed-in state and
requiring it on a path. It does not cover choosing an authentication method,
password storage, or passkeys. Those belong to
[Authentication](/guides/backend/authentication/).
:::

## 1. Install the capability

```sh
pw add auth
```

The wizard asks two things. Answer `auth`, then choose the **local emulator**
when it asks about the OIDC provider. The review screen lists what will be
written before anything is:

```
  Review
    Capability     auth
    OIDC provider  Local emulator

    create  devidp.toml
    create  handlers/accounts.go
    create  migrations/00003_init_popcornwave_session.sql
    create  migrations/00004_init_popcornwave_auth.sql
    append  config.dev.toml
    append  popcornwave.toml
    by hand call handlers.RegisterAccountResolver() in ./cmd/memoapp before pw.Run
    then    pw migrate up
```

Sessions are stored server-side in the database, which is why two migrations
arrive with the login and why `pw add auth` refuses to run in a project without
a database. The browser gets an opaque token; the record it points at can be
expired and revoked from the server.

`devidp.toml` is the roster of development users, `Administrator` and `Member`.
`pw dev` serves them from a local OpenID Provider and checks no password, which
is exactly why it never runs outside development.

The line marked **by hand** is the one edit the command will not make for you.
Open `cmd/memoapp/main.go`:

```go
func main() {
	// Installed before Run: the framework calls it during the OIDC callback.
	handlers.RegisterAccountResolver()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

`RegisterAccountResolver` lives in the `handlers/accounts.go` the wizard just
wrote. The framework verifies an identity with the provider and then asks this
function which local account it belongs to; the starter version derives one from
the identity itself, so the project can log in before it owns an account table.

## 2. Decide what needs a session

`pw add auth` appended an `[auth]` section to `config.dev.toml` with nothing
protected:

```toml
protection.include = []
```

Every path is public until it appears in that list. The memo pages should not
be:

```toml
protection.include = ["/", "/memos"]
protection.unauthenticated = "redirect"
```

Patterns match whole path segments, and they match exactly unless they end in
`**`. `"/"` is the root and nothing below it; `"/memos/**"` would be the subtree.
Listing the two paths this application actually has keeps `/healthz` and the
other operational endpoints public, which is what a load balancer needs them to
be.

While you are in the configuration files, pin the development provider's port in
`popcornwave.toml`:

```toml
[dev.idp]
enabled = true
config = "devidp.toml"
# A fixed port keeps the issuer URL stable across restarts.
port = 18080
```

Without it the provider takes a free port, and the issuer URL changes on every
`pw dev`. The issuer is half of what identifies an account, so a new port would
hand the same person a new account — and, three steps from now, an empty memo
list — after each restart.

Then apply the migrations:

```sh
pw migrate up
```

## 3. Give a memo an owner

The table predates the notion of an owner, so add the column in its own
migration rather than editing the one that has already been applied. Create
`migrations/00005_add_memo_author.sql`:

```sql
-- +goose Up
ALTER TABLE memos ADD COLUMN author TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE memos DROP COLUMN author;
```

The default matters: rows written in chapter 3 have no owner, and `NOT NULL`
without a default cannot be added to a table that already has rows. Those older
memos end up owned by the empty string, which no account is — they stay in the
table and stop appearing on any page.

Both statements in `queries/memos.pw.sql` now take the owner:

```sql
export statement ListMemos(author: string): sql.many<Memo> {
SELECT id, body FROM memos WHERE author = {author} ORDER BY id DESC
}

export statement CreateMemo(author: string, body: string): sql.exec {
INSERT INTO memos (author, body) VALUES ({author}, {body})
}
```

Changing a statement's parameter list changes the generated function's
signature, so every caller stops compiling until it passes the new argument —
the same boundary chapter 1 demonstrated with a template.

## 4. Read the user

The framework resolves the session before any handler runs. `auth.User` reports
what it found:

```go
// handlers/home_handler.go
import (
	"context"
	"net/http"

	"memoapp/queries"

	"github.com/shibukawa/popcornwave/plugin/auth"
	"github.com/shibukawa/popcornwave/pw"
	httpbind "github.com/shibukawa/tinybind-go"
)

func home(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	list, err := listMemos(r.Context(), user.AccountID)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	pw.WriteHTML(w, r, Home(HomeParams{DisplayName: user.DisplayName, Memos: list}))
}

func listMemos(ctx context.Context, author string) ([]Memo, error) {
	var list []Memo
	for row, err := range queries.ListMemos(ctx, author) {
		if err != nil {
			return nil, err
		}
		list = append(list, Memo{Id: row.Id, Body: row.Body})
	}
	return list, nil
}
```

The guard configured in step 2 already redirected every anonymous request away
from this path, so the `!ok` branch should be unreachable. Write it anyway. It
costs three lines, and it means a later edit to `protection.include` cannot
silently turn `user.AccountID` into an empty string that matches every
unowned row.

`createMemo` changes the same way — read the user first, then pass
`user.AccountID` to `queries.CreateMemo` and to the `listMemos` call in the
validation branch:

```go
func createMemo(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	input, err := pw.Parse[createMemoInput](r)
	if err != nil {
		mapped, fieldError := httpbind.AsHTTPError(err)
		if !fieldError || len(mapped.Fields) == 0 {
			pw.WriteProblem(w, r, pw.BadRequest(err))
			return
		}
		list, listErr := listMemos(r.Context(), user.AccountID)
		if listErr != nil {
			pw.WriteProblem(w, r, listErr)
			return
		}
		pw.WriteHTML(w, r, Home(HomeParams{
			DisplayName: user.DisplayName,
			Memos:       list,
			Draft:       r.PostFormValue("body"),
			Error:       mapped.Fields[0].Message,
		}))
		return
	}
	if _, err := queries.CreateMemo(r.Context(), user.AccountID, input.Body); err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

The page takes the name and offers a way out:

```html
package handlers

type Memo {
  id: int
  body: string
}

export component Home(displayName: string, memos: Memo[], draft: string, error: string): html {
<h1>{displayName}'s memos</h1>
<form method="post" action="/auth/logout"><button type="submit">Sign out</button></form>
<form method="post" action="/memos">
  <textarea name="body" rows="3">{draft}</textarea>
  {if error != ''}<p class="error">{error}</p>{/if}
  <button type="submit">Add</button>
</form>
<ul>
{for memo in memos}
  <li>{memo.body}</li>
{/for}
</ul>
}
```

Sign-out is a form rather than a link because the endpoint accepts `POST` only.
A link that ends a session can be triggered by a browser prefetch or an image
tag on someone else's page; a `GET /auth/logout` therefore answers `405`.

## 5. Log in

```sh
pw dev
```

Alongside the usual output, the loop reports the provider it started:

```
devidp: development identity provider on http://127.0.0.1:18080; no password is checked
pw dev: identity provider http://127.0.0.1:18080
pw dev:   login screen http://127.0.0.1:18080/login
pw dev:   client pw-dev-xCD4SA98_as (secret injected as AUTH_OIDC_CLIENT_SECRET)
pw dev:   users admin, member
```

The issuer and the client credentials are injected into the application process
as environment variables, so nothing about the provider is written into a
committed configuration file.

Open <http://127.0.0.1:8080/> — that host, not `localhost`, because the
scaffolded `auth.oidc.redirect_url` names it. The request is redirected to the
provider's login screen, where two users are waiting. Pick **Member**, and the
browser lands back on the memo page with the heading **Member's memos**.

Write a memo. Then sign out, sign in as **Administrator**, and the list is
empty. Sign back in as **Member** and the memo is there.

What happened in between, the application did not implement: the redirect to the
provider, the callback, the verification, the account lookup, the session
record, and the cookie. Three routes came with the capability —
`/auth/login`, `/auth/callback`, `/auth/logout` — and no handler in this project
mentions any of them.

## What is deliberately missing

The development provider is a stand-in. Deploying this application means naming
a real OpenID Provider and supplying `AUTH_OIDC_ISSUER`, `AUTH_OIDC_CLIENT_ID`,
`AUTH_OIDC_CLIENT_SECRET`, and `SESSION_SECRET` through the environment; the
application refuses to start without them rather than falling back to something
insecure.

The starter resolver derives an account from the verified identity instead of
storing one. A real application owns an accounts table, links it to the issuer
and the identity claim, and decides what to do with an identity it has never
seen. [Authentication](/guides/backend/authentication/) covers that, and
`examples/oidclogin` in the repository is a working version of it.

And `auth.User` answers *who*, never *what they may do*. Authorization stays in
the application — including the question this chapter answered with a WHERE
clause.

## What you built

Across four chapters: a project scaffolded and running, a form whose rules are
declared once and enforced before the handler body, a schema under version
control with typed SQL over it, and a login whose protocol work happened
somewhere else entirely.

- [Testing](/productivity/testing/) — handler tests, including a helper that
  completes the whole login flow in one request.
- [Project structure](/guides/architecture/project-structure/) — splitting
  `handlers` once one package stops being enough.
- [Configuration](/guides/architecture/configuration/) — what changes between
  `dev` and a deployed environment.
- [pw build](/pw/project/build/) — producing the binary you deploy.
