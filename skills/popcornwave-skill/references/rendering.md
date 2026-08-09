# Rendering Behaviors

How Popcorn Wave turns templates into responses beyond the basic `pw.WriteHTML` call: discovered routing (the page tree), async rendering, partial updates, live rendering, and form handling. Registered routing means handlers you register in Go; discovered routing means routes generated from a page tree directory. The two share one mux (`pw.ServeMux` is a type alias), and a project can carry either or both. All generated `_pw_gen.go` files are build outputs — never edit them; rerun `pw generate`.

## Discovered routing: a directory is a route

A directory holding `page.pw.html` under the tree root (default `pages/`, set by `generate.pages` in `popcornwave.toml`) is a `GET` route. Nothing registers it by hand — `pw generate` walks the tree and writes the registrations.

```
pages/
├── page.pw.html                → GET /{$}        (root; not bare "/" which would match everything)
├── layout.pw.html
└── users/id_/page.pw.html      → GET /users/{id}
```

Directory naming: a route directory is also a Go package, so dynamic segments are spelled with trailing underscores — `id_/` is `{id}`, `rest__/` is a catch-all `{rest...}` (binds as a string). `[id]`, `{id}`, `:id`, `(group)` etc. are illegal Go import path elements and are rejected by discovery. Directories starting with `_` or `.`, and `testdata`, are ignored.

Wire both routers onto one mux; order does not matter, but registering the same method+path twice panics at startup:

```go
mux := handlers.Handlers()  // registered: your API
pages.Register(mux)         // discovered: the website
```

Scope of the shape: a page is a `GET` that renders a template; its actions are `POST` endpoints. File downloads, webhooks, `PUT`s, and anything that must appear in OpenAPI are registered routes (discovered routes generate no OpenAPI, by design).

### The three rungs

What sits beside `page.pw.html` decides how much Go runs:

| Files | Rung | What you get |
| --- | --- | --- |
| `page.pw.html` only | template | whole handler generated; the template's own `external` calls fetch data |
| `+ page.go` with `func Load(id string) (User, error)` | typed | generated handler decodes the URL, calls `Load`, renders its results |
| `+ page.go` with `func Load(w http.ResponseWriter, r *http.Request)` | handler | only the registration is generated; the response is yours |

The signature picks the rung; a `Load` matching neither shape fails generation. The entry point is named `Load` (not `Page`) because the compiled template already emits `func Page(params PageParams) …` into the same package.

Inputs are ordinary parameters — no struct, no binding tags. Leading parameters are the route's dynamic segments in route order; the rest are query parameters keyed by name. Without `page.go` the list is read from the component; with a typed `Load`, from `Load`, and its result list must match the component's parameter list (count, order, types). Inputs are scalars; a pointer (`page *int`) distinguishes absent from zero:

```go
func Load(id string, page *int) (string, int, error) {
	number := 1
	if page != nil {
		number = *page
	}
	return "user " + id, number, nil
}
```

Use the handler rung when you need the request context (auth, DB pool). It composes its own chain via the generated `Render`:

```go
func Load(w http.ResponseWriter, r *http.Request) {
	route, err := DecodeRoute(r)
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	if err := Render(w, r, route, PageParams{}); err != nil {
		pw.WriteProblem(w, r, err)
	}
}
```

(Or simply `pw.WriteHTML(w, r, Page(PageParams{…}))` after loading data — see the tutorial's archive page.)

### Layouts

An ancestor `layout.pw.html` wraps every page below it, outermost first. It must declare `children: html` (that shape is what generates the wrapper binder), and it may only read dynamic segments at or above its own directory:

```html
package pages

export component Layout(children: html): html {
<div class="page"><slot required /></div>
}
```

The document shell is still outermost: `templates/document.pw.html` owns doctype/`<head>`/`<body>` and wraps the layout chain from outside. A `document.pw.html` placed inside the tree is not applied.

### Server actions

A template element names an exported Go handler; generation resolves it to a URL:

```html
<button server-action="Rename" data-target="#name">rename</button>
<!-- lowers to -->
<button data-pw-action="/_action/00369cf962b6/Rename" data-target="#name">rename</button>
```

```go
func Rename(w http.ResponseWriter, r *http.Request) { /* owns the whole response */ }
```

Every exported handler-shaped function in a route package gets a `POST /_action/<hash>/<Name>` endpoint (lowercase it to keep it private; `Load` is excluded). Addresses are deterministic across builds. An address grants nothing — each handler still authenticates and authorizes. The generated registry exposes `Routes` and `Actions` tables for inspection.

Caveats (documented as not there yet): the client runtime that acts on `data-pw-action` does not exist — intercepting the click is yours, and custom action code must copy the `pw_csrf` cookie into the `X-CSRF-Token` header with the CSRF middleware enabled over the action paths. A site built entirely from the template and typed rungs works with no JavaScript at all.

Commands: `pw init mysite --router=discovered` (or `registered` / `both`), `pw add discovered` / `pw add registered` to install the other later, `pw new page` to add a route interactively. `pw dev` picks up new routes automatically.

**When not to use it:** anything that is not a GET page or its POST action — downloads, webhooks, non-GET APIs, endpoints that must appear in OpenAPI. Those are registered routes.

## Async rendering

An async page streams: the shell, the head, every settled value, and every `fallback` commit first; each slow section replaces its placeholder as its data settles — one HTTP response, no client-side data fetching.

Handler side — pass pending values instead of finished ones. No streaming API, no header, no flush; `pw.WriteHTML` streams only when the composed document holds an `await` boundary:

```go
pw.WriteHTML(w, r, Home(HomeParams{
	Profile:        Profile{Name: "Ada Lovelace", Joined: "2026-02-11"},
	Orders:         pw.Go(ctx, loadOrders),
	Recommendation: pw.Go(ctx, recommend),
}))
```

| Constructor | Use |
| --- | --- |
| `pw.Go(ctx, work)` | start the work now, in its own goroutine (`work` is `func(context.Context) (T, error)`) |
| `pw.Resolved(v)` | a value you already have, and tests |
| `pw.Failed(err)` | a failure you already know about |

A handle settles once and stays readable (a layout and a page may await the same handle; the work runs once). There is no channel constructor — receive from the channel inside the `pw.Go` closure. Starting early is the point: work overlaps everything rendered above it.

Template side — declare the parameter `async` and read it in an `await` block (syntax in references/templates.md):

```html
export component Home(profile: Profile, orders: async Order[]): html {
<h1>{profile.name}</h1>
{await list = orders}
  <ul>{for order in list}<li>{order.id} — {order.total}</li>{/for}</ul>
{fallback}
  <p class="pending">Loading orders…</p>
{/await}
}
```

Failure: with `recover`, the failure is contained to that section and the response stays 200 (the status left with the shell). Without `recover`, the framework replaces everything below the document shell with the page registered via `pw.RegisterHTMLErrorPage` — a `recover` subtree never sees a raw Go error (default `code: "internal"`; implement `PublicError() pw.AsyncError` for a safe projection). With `streaming = false` the same failure answers a real 500.

The runtime script: the document shell loads one small ES module that swaps each completion (`<template data-tb-boundary>` + `<tb-apply>`) into its `<tb-boundary>` placeholder. No inline script travels in the stream, so `script-src 'self'` suffices. The scaffolded shell references it through an external function because the URL carries a content-derived revision:

```html
external RuntimeScriptURL(): url

export component Document(children: html?): html {
<!doctype html>
<html><head>...<script type="module" src={RuntimeScriptURL()}></script></head>
<body><slot /></body></html>
}
```

```go
func RuntimeScriptURL() *url.URL { return &url.URL{Path: pw.RuntimeScriptURL()} }
```

`pw init` scaffolds both halves.

Clients without the runtime: crawlers and CLI clients (known bot names, plus any `User-Agent` not starting `Mozilla`, plus missing `User-Agent`) get the buffered, complete document — and truthful error statuses. A browser with scripting disabled is redirected once via a contributed `<noscript>` block to a buffered render of the same URL. `pw.IsBot(r)` may pick a cheaper query but must never change content or reach an access decision. Any streamable page carries `Vary: User-Agent`.

Configuration (`popcornwave.toml`, `[html]`): `streaming` (false forces buffered), `async_timeout` (default bound per boundary; expiry renders `recover` with `code:"timeout"`), `async_concurrency`, `bot_detection`, `bot_async_timeout` (default 5s), `bot_user_agents`, `scriptless_detection`.

**When not to use it:** a page whose data is already fast — a boundary adds a placeholder, a swap, and a connection held open until the slowest boundary settles (`async_timeout` is a resource bound). Fragments never stream. Write fallbacks as an enhancement over content that is already meaningful.

## Partial updates

The same URL answers a complete document to anything that asks for one, and to a page that already holds the layout it answers only the boundaries whose markup changed. Every layout and page of a rendered chain is already an update boundary (ordinary components deliberately are not). Handlers and templates are unchanged.

```toml
[html.update]
enabled = true
validator_key = "${HTML_UPDATE_VALIDATOR_KEY}"
```

Startup refuses `enabled = true` with no key (digests are keyed). Off by default: it adds a browser runtime to every document, a secret to deploy, and a rule that every re-render must be free of side effects.

Three paths, chosen by who holds the changed input:

- **Navigation** — input lives in the URL (sort, filter, page number). The runtime intercepts same-origin links and GET forms and the server sends back only changed boundaries. Zero code. **State that can live in the URL belongs there.**
- **Redraw** — input the browser holds and that should not be in a shareable URL. Annotate the component:

  ```html
  @reloadable
  export component OrderCard(id: string, orderID: int): html {
  <article class="card">
    <h3>Order {orderID}</h3>
  </article>
  }
  ```

  Requirements: exported, exactly one root element, a required `id` parameter, every other parameter a scalar a query string carries (records, slices, `html` are errors). A page-tree page needs no handler code; a registered handler can answer a redraw before its data load, below its auth check:

  ```go
  if !pw.Authenticated(r.Context()) { pw.WriteProblem(w, r, pw.Unauthorized()); return }
  if pw.Redraw(w, r, templates.OrdersPage) { return }   // page is named, not called
  ```

  A redraw's arguments come from the caller — a component loading a record by id must check ownership itself.
- **Action** — a mutation. Keep the mutation above the branch; clients without the runtime take the redirect:

  ```go
  if !pw.WantsUpdate(r) {
  	http.Redirect(w, r, "/orders", http.StatusSeeOther)
  	return
  }
  pw.WriteUpdate(w, r, http.StatusOK,
  	pw.Replace("order-summary", templates.Summary(templates.SummaryParams{Order: order})))
  ```

  A rejected submission returns 4xx and its regions are the validation errors. When the user belongs elsewhere: `pw.WriteUpdateNavigate(w, r, "/orders/17")`.

Browser API (feature-detect; updates may be disabled): `window.popcornwave.update({sort:"newest"})` (takes the whole query — unnamed params are dropped), `.redraw("card-17", {orderID: 17})`, `.updateHeaders()` + `.apply(response)` around a `fetch`. `data-tb-ignore` hands an element back to the browser; non-GET submissions, modified clicks, `target`, `download`, and cross-origin URLs always stay the browser's. `data-tb-preserve="name"` moves a region the server does not own (map, canvas, video) into the replacement. During an open update the root carries `data-tb-updating` — style it for a progress indicator.

Pitfalls: re-renders may be discarded, so rendering must be side-effect free (mutations belong in actions); a boundary embedding the clock ("3 seconds ago") never matches and is re-sent every time; a GET update cannot clear a form back to defaults — post-redirect-get clears through a page load.

**When not to use it:** a page that reloads in ~100 ms needs none of this. When the application wants to own the swapping (a dialog filled from a chosen route), use `pw.WriteHTMLFragment` instead. When the *server* learns something new without a request, use live rendering.

## Live rendering

A live source keeps producing, its boundary re-renders per value, and the browser holds one connection back to the page's own URL. Template syntax is unchanged — a live source binds in the same `await` clause as an async one, and switching a source `async` → `live` changes no calling template. The handler changes nothing: generated code calls the source with the subscription's context.

```html
external live WatchMetrics(id: string): Point
```

```go
func WatchMetrics(ctx context.Context, id string) iter.Seq2[Point, error] {
	return func(yield func(Point, error) bool) {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				point, err := sample(ctx, id)
				if !yield(point, err) {
					return
				}
			}
		}
	}
}
```

Rules of the shape:

- The `context.Context` is **mandatory** — it is the only thing that ends an endless source.
- A yielded error is a delivery, not the end: the boundary shows `recover`, and the next good value restores primary content. The sequence returning is what ends the subscription.
- Pulling is the backpressure: one value at a time; a fast source blocks in `yield` and misses ticks.
- **A delivery is the whole region, not the change.** Yield the current list, not the new message; the source owns accumulation. A long list pays its length per delivery — keep the boundary around what actually changes.

The first response is an ordinary async stream: the first delivery completes the boundary and the document finishes, so crawlers and script-less browsers see one real render. A source producing nothing within `html.async_timeout` leaves the fallback — not a failure, nothing logged. Afterwards the runtime opens one connection (`GET` same URL with `Pw-Response-Mode: live`); the route executes again — same auth, same binding — writes no body, and streams one NDJSON record per delivery. Reconnects re-run the page; deploys and unknown boundary ids trigger one guarded reload.

A live region renders **output, not input**: `form`, `input`, `textarea`, `select`, or `contenteditable` in the primary subtree is a generation error. Put ARIA live-region attributes (`role="log"`) on an element **around** the boundary, not inside it.

Configuration under `[html]` (all require `streaming = true`): `live` (false is a safe dial — documents stay valid, no client connects), `live_max_duration` (default 10m; bounded lifetime with `live_duration_jitter`), `live_idle_timeout`, `live_max_boundaries`, `live_max_responses`. Keep `middleware.request_timeout` above `live_max_duration` or at zero.

**When not to use it:** rendering is per client (ten screens = ten renders per tick) and each reconnect is a full page execution — count page executions per second, not connections. Share upstreams inside the source. For occasional freshness, a five-second poll or a plain reload is cheaper; for user-initiated changes, use partial updates.

## Forms

Baseline pattern — a plain POST with server-side rules, then post-redirect-get:

```html
<form method="post" action="/memos" class="mt-6 space-y-2">
  <textarea name="body" rows="3" required maxlength="200"></textarea>
  <button type="submit">Add</button>
</form>
```

```go
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
	http.Redirect(w, r, "/", http.StatusSeeOther) // 303: reload re-reads, never resubmits
}
```

- The unsafe form's CSRF token is generated as its first hidden child — write nothing (see references/templates.md).
- HTML attributes (`required`, `maxlength`, `min`/`max`, `pattern`, input types) echo the `check` rules for immediate feedback; the server's `check` rules are the truth and must exist regardless. Never make an attribute narrower than its check.
- `pw.WriteProblem` answers a failed check as RFC 9457 problem JSON, or as the scaffolded `templates/400.pw.html` when `Accept` prefers HTML — no branch in the handler.

Re-rendering with the reader's text: `pw.Parse` returns the zero value on failure, so read the raw fields back:

```go
input, err := pw.Parse[createInput](r)
if err != nil {
	mapped, ok := httpbind.AsHTTPError(err)
	if !ok || len(mapped.Fields) == 0 {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	form := FormState{Title: r.PostFormValue("title"), Owner: r.PostFormValue("owner")}
	applyFieldErrors(&form, mapped.Fields)
	pw.WriteHTML(w, r, NewTask(NewTaskParams{Form: form}))
	return
}
```

Style failures with `:user-invalid` (not `:invalid`, which matches untouched empty fields), and remember scoped selectors need a class to hang off.

Swap-library (htmx-style) path: answer with `pw.WriteHTMLFragment` — no shell, no merged head, never streams. A rejected form on this path answers **HTML with a 200**, because swap libraries do not swap non-2xx responses. Two routes (page + fragment) are usually clearer than one route branching on `HX-Request`. `examples/htmx_fragment` is the complete reference application.

**When not to use each:** plain PRG covers most forms — do not add a runtime for it. Reach for the partial-update action branch when a whole-page reload is what people notice; reach for fragments when the application owns the swap (a dialog that must stay open showing errors). Forms never belong inside a live boundary — that is a generation error.
