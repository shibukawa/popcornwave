---
title: 2. Forms and validation
description: Accept a POST, declare the rules the input must satisfy, and report a rejected form to the person who filled it in.
sidebar:
  order: 2
---

Chapter 1 left a project that greets you and reloads when you save. Reading a
query parameter is one thing; accepting something a person typed is another,
because a person can type nothing at all, or two hundred lines of it.

This chapter adds a form. The memos live in memory for now — chapter 3 moves
them into a table — so the whole change is three files and about twenty
minutes.

:::note[Where this starts]
From chapter 1: `pw init memoapp`, and `handlers/home.pw.html` back at the
scaffolded `Home(name: string, project: string)`. If you skipped chapter 1, `pw init memoapp`
produces exactly that state.
:::

## 0. Add Tailwind

Chapter 1 declined Tailwind. Install it now:

```sh
pw add tailwind
```

A wizard opens, and before anything is written it shows a review screen listing
every file it will create and every file it will append to. There is no flag
that skips it: this command edits a project that already exists. Accept it and
the pinned toolchain, `assets/app.css`, and the CSS build step go in.

This is what "declined at init is not permanent" means in practice. Chapter 3
adds the database the same way, and chapter 4 the login.

## 1. A page with a form on it

The page has two jobs now: list what has been written, and offer a box to write
the next one. Replace `handlers/home.pw.html`:

```html
// handlers/home.pw.html
package handlers

type Memo {
  id: int
  body: string
}

export component Home(memos: Memo[], draft: string, error: string): html {
  <h1 class="text-3xl font-bold">Memos</h1>
  <form method="post" action="/memos" class="mt-6 space-y-2">
    <textarea name="body" rows="3"
      class="w-full rounded-lg border border-slate-300 p-3 focus:border-indigo-500 focus:outline-none">{draft}</textarea>
    {if error != ''}<p class="text-sm text-red-600">{error}</p>{/if}
    <button type="submit"
      class="rounded-lg bg-indigo-600 px-4 py-2 font-medium text-white hover:bg-indigo-500">Add</button>
  </form>
  <ul class="mt-8 space-y-2">
  {for memo in memos}
    <li class="rounded-lg border border-slate-200 p-3">{memo.body}</li>
  {/for}
  </ul>
}
```

Three things arrived at once. `type Memo` declares a composite that becomes a Go
struct, so the rows the template renders and the rows the handler builds are one
type rather than two that have to be kept in step. `Memo[]` is a slice of it.
And `draft` and `error` exist for a case that has not been written yet: a
submission that comes back rejected, with the text still in the box.

Conditions in `.pw.html` are booleans — there is no truthiness — which is why
the error test is `error != ''` rather than `error`.

## 2. Somewhere to put them

Memos need to survive from one request to the next. A slice behind a mutex is
enough for one chapter:

```go
// handlers/memos.go
package handlers

import "sync"

// memos keeps the list in memory. Chapter 3 replaces it with a table.
var memos = &store{}

type store struct {
	mu     sync.Mutex
	nextID int
	items  []Memo
}

func (s *store) list() []Memo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Memo(nil), s.items...)
}

func (s *store) add(body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	s.items = append([]Memo{{Id: s.nextID, Body: body}}, s.items...)
}
```

`Memo` here is the type the template declared. Nothing converts between a
presentation type and a storage type, because at this size there is only one
type. Chapter 3 introduces the second one, and the conversion that goes with it.

## 3. Two routes

```go
// handlers/home_handler.go
package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

func init() {
	mux.HandleFunc("GET /{$}", home) // changed: the scaffold had "GET /"
	mux.HandleFunc("POST /memos", createMemo) // new
}

// changed: the scaffolded home read a homeInput through pw.Parse.
// There is nothing left to read, so that type goes with it.
func home(w http.ResponseWriter, r *http.Request) {
	pw.WriteHTML(w, r, Home(HomeParams{Memos: memos.list()}))
}

// Everything below is new; none of it was in the scaffold.
type createMemoInput struct {
	Body string `payload:"body" check:"required,maxlen=200"`
}

func createMemo(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[createMemoInput](r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	memos.add(input.Body)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

The scaffolded pattern was `GET /`, which in Go's mux matches every path that
nothing more specific claims. `GET /{$}` matches the root and only the root, so
a typo in the URL now gets a 404 instead of the memo list.

`payload:"body"` reads the request body rather than the query string; the form
posts `application/x-www-form-urlencoded`, and the same declaration would accept
a JSON body from an API client without a second code path. The `check` rules are
compiled during generation, so `required` and `maxlen=200` cost no reflection at
request time. [Handlers](/guides/frontend/handlers/#validation) lists every rule.

The redirect is a `303 See Other`, and it matters more than it looks. A POST
that answers with a page leaves the browser holding a resubmittable request:
reload, and the memo is added twice. Answering with a redirect to `GET /` means
the reload re-reads the list instead.

Save the files. `pw dev` regenerates, rebuilds, and restarts; type something
into the box and press **Add**, and it appears in the list.

## 4. Submit an empty form

Press **Add** with the box empty. The browser shows this:

```json
{
  "code": "validation_failed",
  "detail": "Validation failed",
  "errors": [{ "field": "body", "location": "payload", "message": "required" }],
  "status": 400,
  "title": "Validation failed",
  "type": "about:blank"
}
```

That is a correct answer, and it is the right one for an API client: RFC 9457
problem details, status 400, the offending field named. `pw.WriteProblem` maps a
binding failure to exactly this, and if you had wrapped the error —
`pw.BadRequest(err)`, as the scaffolded handler does — the status would be the
same but the `errors` array would be gone, since the wrapper replaces the
validation error rather than carrying it.

For a person who mistyped a form, though, a JSON document is a dead end. What
they need is the page again, with the message next to the field and their text
still in it. The declarations stay where they are; the handler decides what to
do with the failure:

```go
// This replaces the createMemo of section 3.
import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
	httpbind "github.com/shibukawa/tinybind-go" // new
)

func createMemo(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[createMemoInput](r)
	if err != nil {
		mapped, fieldError := httpbind.AsHTTPError(err)
		if !fieldError || len(mapped.Fields) == 0 {
			// Not a field-level failure — an unreadable body, or one too
			// large. Nothing on the page can be usefully re-rendered for it.
			pw.WriteProblem(w, r, pw.BadRequest(err))
			return
		}
		pw.WriteHTML(w, r, Home(HomeParams{
			Memos: memos.list(),
			Draft: r.PostFormValue("body"),
			Error: mapped.Fields[0].Message,
		}))
		return
	}
	memos.add(input.Body)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
```

`httpbind.AsHTTPError` is the test that separates the two situations. A failed
`check` is expected traffic — someone submitted an empty box — and belongs on
the page. A body that could not be read at all is not, and still answers with a
problem document.

The re-rendered text comes from `r.PostFormValue`, not from `input`: when a
check fails, `pw.Parse` returns the zero value, so the only place the typed
characters still exist is the request itself. Binding already parsed that body,
so reading it again costs nothing.

Save, submit an empty form again, and the page comes back with `required` under
the box. Submit more than two hundred characters and the message changes
accordingly.

One honest limitation: `pw.WriteHTML` takes no status code and answers `200`.
The response above is a page about a rejected input, sent with a success status.
For a browser form that is normal — the browser renders it either way — but an
API client on the same route needs the problem document, which is what the JSON
branch is for.

## What you have now

- A form that posts, a route that accepts it, and a redirect that survives a
  reload.
- Validation declared on the struct and compiled ahead of the request.
- One failure, answered two ways: a problem document for a client, a re-rendered
  page for a person.

The list still disappears on every restart. Chapter 3 gives it a table.

- [3. Storing the memos](/tutorial/database/) — the next chapter.
- [Handlers](/guides/frontend/handlers/) — every source tag and check rule.
- [Responses](/guides/frontend/responses/) — problem details, JSON, and fragments.
