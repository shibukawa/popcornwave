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
    edit    cmd/memoapp/main.go
    then    pw migrate up
```

`devidp.toml` is the roster of development users, `Administrator` and `Member`.
`pw dev` serves them from a local OpenID Provider and checks no password, which
is exactly why it never runs outside development.

### A login needs two kinds of storage

Two migrations is not an accident. A login puts two different things on the
server:

| What | Holds | Package |
| --- | --- | --- |
| Sessions | who is signed in | `sessionstore/sqlite` |
| Ceremony records | the single-use state of one login in progress | `authstate/sqlite` |

![auth runs once at sign-in, takes the browser out to an external IdP and back, and hands session an account ID; session carries it on every request after that. Below auth sit the external IdP and authstate; below session sits sessionstore](../../../assets/diagrams/auth-and-session.svg)

**auth decides who this is.** It sends the browser out to a provider, verifies
the identity that comes back, and settles on an account ID for this application.
It runs at sign-in and not again.

**session carries who it decided.** On every request after that, it holds who
this is and what is true of them right now.

A session is the record behind the opaque token the browser carries, which is
what makes it revocable from the server. A ceremony record correlates the one
round trip out to the provider and back, and is consumed when it returns.

`session.backend = "rdb"` in `config.dev.toml` chooses only **where** they go.
**What puts that somewhere into the binary is the import.** That is what "storage
is opt-in" means: an application links the backend it configured and no other, and
a project keeping sessions in a cookie links no store at all — see
[Cookies](/guides/backend/cookies/), which `pw init --session` offers from the
start.

When the configuration and the imports disagree, startup says so:

```
popcornwave: auth.session: session.backend = "rdb" needs its plugin;
add to the application: import _ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
```

That is the `edit cmd/memoapp/main.go` line. Accept the screen and the file
becomes this:

```go
// cmd/memoapp/main.go
package main

import (
	"context"
	"log"

	"memoapp/handlers"

	// new: this is what actually provides session.backend = "rdb".
	_ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
	// new: where the single-use login records go.
	_ "github.com/shibukawa/popcornwave/authstate/sqlite"

	"github.com/shibukawa/popcornwave/pw"
)

func main() {
	// new: installed before Run, because the framework calls it during the
	// OIDC callback.
	handlers.RegisterAccounts()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

A project that chose the login at `pw init` has these three lines in its
scaffold already, which is exactly why `pw add` writes them too: a capability
declined at bootstrap and installed later has to arrive at the same file as one
that was never declined.

`main.go` belongs to the application, so editing it is not something a command
should do quietly. What makes it acceptable is that the review screen names the
file before anything is written, and that the edit is spliced in at a position
the parser found — the rest of the file is copied through unchanged, comments,
grouping, and all.

`RegisterAccounts` lives in the `handlers/accounts.go` the wizard just wrote.
It is one line — `auth.SetAccountResolver(resolveAccount)` — and it installs
whichever account seams the selected mode needs. The framework verifies an
identity with the provider and then asks `resolveAccount` which local account it
belongs to; the starter version derives one from the identity itself, so the
project can log in before it owns an account table.

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

In `popcornwave.toml`, what `pw add auth` wrote is already what you want:

```toml
[dev.idp]
enabled = true
config = "devidp.toml"
port = 18080
```

The `port` is deliberate. Without it the provider takes a free port and the
issuer URL changes on every `pw dev`. The issuer is half of what identifies an
account: the `resolveAccount` you will read in section 4 builds an account ID out
of `issuer + "|" + subject`. A moving port would hand the same person a new
account after every restart — and, three steps from now, an empty memo list. If
something already listens on 18080, change it.

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
	"net/http"

	"memoapp/queries"

	"github.com/shibukawa/popcornwave/plugin/auth" // new
	"github.com/shibukawa/popcornwave/pw"
)

// home lists the signed-in account's memos.
//
// changed: the home of chapter 3, plus the user lookup and the author filter.
func home(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	var list []Memo
	// changed: the author is a new argument, and it is what makes this one
	// person's list rather than everyone's.
	for row, err := range queries.ListMemos(r.Context(), user.AccountID) {
		if err != nil {
			pw.WriteProblem(w, r, err)
			return
		}
		list = append(list, Memo{Id: row.Id, Body: row.Body})
	}
	pw.WriteHTML(w, r, Home(HomeParams{DisplayName: user.DisplayName, Memos: list}))
}
```

The guard configured in step 2 already redirected every anonymous request away
from this path, so the `!ok` branch should be unreachable. Write it anyway. It
costs three lines, and it means a later edit to `protection.include` cannot
silently turn `user.AccountID` into an empty string that matches every
unowned row.

`createMemo` changes the same way — read the user first, then pass
`user.AccountID` to `queries.CreateMemo`:

```go
// handlers/home_handler.go

// createMemo stores one memo as the signed-in account's own.
func createMemo(w http.ResponseWriter, r *http.Request) {
	// changed: these two lines are what decides whose memo this is.
	user, ok := auth.User(r.Context())
	if !ok {
		pw.WriteProblem(w, r, pw.Unauthorized())
		return
	}
	input, err := pw.Parse[createMemoInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	// changed: the author is a new argument.
	if _, err := queries.CreateMemo(r.Context(), user.AccountID, input.Body); err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

The page takes the name and offers a way out:

```html
// handlers/home.pw.html
package handlers

type Memo {
  id: int
  body: string
}

export component Home(displayName: string, memos: Memo[]): html {
  <h1 class="text-3xl font-bold">{displayName}'s memos</h1>
  <form method="post" action="/auth/logout"><button type="submit">Sign out</button></form>
  <form method="post" action="/memos" class="mt-6 space-y-2">
    <textarea name="body" rows="3" required maxlength="200"
      class="w-full rounded-lg border border-slate-300 p-3"></textarea>
    <button type="submit"
      class="rounded-lg bg-indigo-600 px-4 py-2 font-medium text-white">Add</button>
  </form>
  <ul class="mt-8 space-y-2">
  {for memo in memos}
    <li class="rounded-lg border border-slate-200 p-3">{memo.body}</li>
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

Open <http://localhost:8080/> — the host `pw dev` printed, and the one the
scaffolded `auth.oidc.redirect_url` names. Both matter and they are the same
fact: a login begun at one origin returns to another origin's cookies, so the
callback would arrive holding nothing to check and be refused. The request is
redirected to the
provider's login screen, where two users are waiting. Pick **Member**, and the
browser lands back on the memo page with the heading **Member's memos**.

![the development identity provider offering Administrator and Member accounts and warning that no password is checked](../../../assets/screenshots/tutorial-login.png)

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

All of it is an ordinary server-rendered application, and nothing so far needed
a template language of its own to build.
[Chapter 5](/tutorial/page-tree/) is where that changes: a route the filesystem
describes, and the three ways one of its pages keeps changing after it has been
rendered.
