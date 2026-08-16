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
them into a table — so the whole change is three files and about half an hour,
with a stop halfway to look at what the first of them does before the other two
exist.

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

export component Home(memos: Memo[]): html {
  <h1 class="text-3xl font-bold">Memos</h1>
  <form method="post" action="/memos" class="mt-6 space-y-2">
    <textarea name="body" rows="3" required maxlength="200"
      class="w-full rounded-lg border border-slate-300 p-3 focus:border-indigo-500 focus:outline-none"></textarea>
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

`type Memo` declares a composite that becomes a Go struct, so the rows the
template renders and the rows the handler builds are one type rather than two
that have to be kept in step. `Memo[]` is a slice of it.

`required` and `maxlength="200"` are the form declaring what it will accept —
the same rules the `check` tag on `createMemoInput` will declare in a moment,
said to the browser as well. Press the button with the box empty, or paste more
than two hundred characters, and no request leaves.

That does not replace the validation on the server. **The form is a
convenience; the handler is the boundary.** Writing both is not duplication:
they stop different things, and section 5 is where the difference shows.

## 2. Make it compile, then look at it

Save the template and the build stops:

```
handlers/home_handler.go:21:37: unknown field Name in struct literal of type HomeParams
```

Chapter 1 produced that error on purpose. This time it is incidental: the
component takes memos now and nothing takes a name, so the handler is calling a
function that no longer exists in that shape. Give it the argument it can supply
today, which is none:

```go
// handlers/home_handler.go — home, until section 3 gives it something to list
func home(w http.ResponseWriter, r *http.Request) {
	pw.WriteHTML(w, r, Home(HomeParams{}))
}
```

Delete `homeInput` and the `pw.Parse` call with it. Nothing reads the query
string any more.

The page is now half a feature — a form that posts to a route that does not
exist — and it is worth stopping here anyway, because two of the three things
you just wrote can already be checked.

**The template, on its own.** Open the console at
<http://127.0.0.1:18081>, take the **storybook** pane, and select `Home`. It
renders with no application involved: two list items, both reading `Body`,
because the harness makes parameters up from their type and a string field is
worth its own name. Edit the parameters under the story and render again to ask
it a different question. That is the answer to "does this template do what I
think" without a handler, a route, or a row anywhere.

![a template rendered on its own, with tabs for rendered and HTML output and an editable parameter block below](../../../assets/screenshots/dev-console-story.png)

**The page, in the browser.** Reload <http://localhost:8080/>. The form is there
and the list below it is empty, which is what an empty slice renders as. Type
something and press **Add**:

```
404 page not found
```

Nothing is broken. `POST /memos` is a route no one has registered, so the mux
has nothing to match and says so. Section 4 registers it.

## 3. Somewhere to put them

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

## 4. Two routes

```go
// handlers/home_handler.go
package handlers

import (
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

func init() {
	mux.HandleFunc("GET /{$}", home)
	mux.HandleFunc("POST /memos", createMemo) // new
}

// homeInput and its pw.Parse call went in section 2: nothing on this page
// reads the query string any more.

// home lists every memo that has been written.
//
// changed: the empty HomeParams of section 2 now carries the store's list.
func home(w http.ResponseWriter, r *http.Request) {
	pw.WriteHTML(w, r, Home(HomeParams{Memos: memos.list()}))
}

// Everything below is new; none of it was in the scaffold.

// createMemoInput is the submitted form.
type createMemoInput struct {
	// Body is the memo text. It is required and capped at 200 characters.
	Body string `payload:"body" check:"required,maxlen=200"`
}

// createMemo stores one memo and redirects back to the list.
//
// A rejected submission comes back as the same page with the message beside
// the field, not as a problem document.
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

`GET /{$}` is the scaffolded pattern, unchanged. A bare `GET /` in Go's mux
matches every path that nothing more specific claims; the `{$}` restricts it to
the root, so a typo in the URL gets a 404 rather than the memo list.

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
into the box and press **Add**. The 404 is gone, the browser follows the
redirect back to `/`, and the memo is at the top of the list. Add a second one
and reload the page: both survive, because they are in a process that has not
restarted. Save any file to make it restart, and they are gone — which is
chapter 3's whole subject.

![the memo form after two submissions, with a textarea, an Add button, and the two newest memos listed below](../../../assets/screenshots/tutorial-forms.png)

## 5. When it is rejected

Press **Add** with the box empty. **Nothing happens.** The browser refuses to
submit and puts its own message under the box, because the textarea says
`required`. Paste more than two hundred characters and it stops accepting them
at the two hundredth.

That is not the end of it. What the form stops is submissions from the form.
Reach the same route without a browser:

```sh
curl -i -X POST http://localhost:8080/memos \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data 'body='
```

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

That is `check:"required"` doing its half: RFC 9457 problem details, status 400,
the offending field named. **Delete `required` from the form and this answer does
not change.** Neither one was redundant — they stop different things. The form
catches a person mistyping; `check` looks at the request.

Now do it again as a browser would:

```sh
curl -i -X POST http://localhost:8080/memos \
  -H 'Accept: text/html' \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data 'body='
```

```
HTTP/1.1 400 Bad Request
Content-Type: text/html; charset=utf-8
Vary: Accept
```

The same failure, answered as a page. `pw.WriteProblem` reads `Accept` and picks
the representation: `templates/400.pw.html` for a client that wants a page,
problem details for everything else. Notice that the handler has no branch in
it. One route answers a browser and an API client, and none of the code that
does it is yours.

That page is `templates/400.pw.html`, which `pw init` left behind. Open it and
you will find it takes the status, the title, the detail, and the field failures
as parameters. What fills them in is decided by the environment:

- `dev`: everything the problem carries. The reader is the developer who caused
  it and is about to fix it.
- anywhere else: the status, the title, and the request id. The same page served
  to the public says what went wrong and not why.

One template, and what changes is what it is handed. Restart under `APP_ENV=prod`
and the same curl comes back with the details gone.

:::note[Where the godoc you wrote ends up]
With `pw dev` still running, open <http://localhost:8080/docs>. Both routes from
this chapter are listed. The summary on `POST /memos` is the first sentence of
`createMemo`'s godoc, the text under it is the rest, and the description on the
`body` parameter is the field comment in `createMemoInput`.

`pw generate` copied all of it into the OpenAPI document. Skip the godoc and the
page lists paths and types and nothing else — the code runs the same either way,
but whether anyone else can read this API does not. Add a sentence, save, and it
is there.

The reference UI comes from `server.api_doc` in `config.dev.toml`. Leave that key
out of a production configuration and the document is still generated, just not
served.
:::

## What you have now

- A form that posts, a route that accepts it, and a redirect that survives a
  reload.
- Validation in two places: the form stops a person mistyping, the `check` rules
  stop the request.
- Two representations of one failure, chosen by the framework from `Accept`, with
  no branch in the handler.

The list still disappears on every restart. Chapter 3 gives it a table.

- [3. Storing the memos](/tutorial/database/) — the next chapter.
- [Handlers](/guides/frontend/handlers/) — every source tag and check rule.
- [Responses](/guides/frontend/responses/) — problem details, JSON, and fragments.
