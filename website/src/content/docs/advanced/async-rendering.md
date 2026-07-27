---
title: Async Rendering
description: Stream a page whose slow sections arrive after the rest of it, using async template parameters and pw.Go.
sidebar:
  order: 1
---

A page is usually as slow as its slowest query. The handler waits for everything,
the template renders once, and the reader sees nothing until the last dependency
answers.

Async rendering breaks that coupling. The parts that are ready commit
immediately, and each slow section replaces its own placeholder as its data
settles — over one HTTP response, with no client-side data fetching.

## Motivation

Consider a page with a profile you already have, an order list behind a 900 ms
query, and a recommendation behind a 1500 ms call.

Rendered normally, the reader waits 1.5 seconds for a blank tab, then gets
everything at once. Rendered asynchronously, the shell and the profile arrive in
20 ms, the orders at 0.9 s, and the recommendation at 1.5 s. The total is still
1.5 s, because the two dependencies overlap rather than queue — but the page
became useful 75× earlier.

<figure>
<svg viewBox="0 0 700 210" role="img" aria-label="Timeline of one streamed response. The shell and both fallbacks are delivered at 20 milliseconds. The orders arrive at 0.9 seconds and the recommendation at 1.5 seconds, having run concurrently rather than one after the other.">
  <g fill="currentColor" font-size="12" font-family="inherit">
    <text x="0" y="26" opacity="0.75">shell + fallbacks</text>
    <text x="0" y="70" opacity="0.75">orders</text>
    <text x="0" y="114" opacity="0.75">recommendation</text>
  </g>
  <g fill="currentColor">
    <rect x="150" y="14" width="10" height="16" rx="2"/>
    <rect x="150" y="58" width="272" height="16" rx="2" opacity="0.18"/>
    <rect x="422" y="58" width="10" height="16" rx="2"/>
    <rect x="150" y="102" width="460" height="16" rx="2" opacity="0.18"/>
    <rect x="610" y="102" width="10" height="16" rx="2"/>
  </g>
  <g fill="currentColor" font-size="11" font-family="inherit" opacity="0.75">
    <text x="172" y="27">delivered</text>
    <text x="256" y="70">waiting on the query</text>
    <text x="330" y="114">waiting on the call</text>
  </g>
  <line x1="150" y1="8" x2="150" y2="150" stroke="currentColor" stroke-width="1" stroke-dasharray="3 3" opacity="0.5"/>
  <text x="150" y="168" fill="currentColor" font-size="11" font-family="inherit" text-anchor="middle" opacity="0.9">readable</text>
  <line x1="150" y1="140" x2="640" y2="140" stroke="currentColor" stroke-width="1" opacity="0.35"/>
  <g stroke="currentColor" stroke-width="1" opacity="0.35">
    <line x1="150" y1="140" x2="150" y2="146"/>
    <line x1="303" y1="140" x2="303" y2="146"/>
    <line x1="457" y1="140" x2="457" y2="146"/>
    <line x1="610" y1="140" x2="610" y2="146"/>
  </g>
  <g fill="currentColor" font-size="11" font-family="inherit" text-anchor="middle" opacity="0.6">
    <text x="303" y="168">0.5s</text>
    <text x="457" y="168">1.0s</text>
    <text x="610" y="168">1.5s</text>
  </g>
  <text x="150" y="196" fill="currentColor" font-size="11" font-family="inherit" opacity="0.6">Both dependencies overlap, so the total is 1.5s rather than 2.4s.</text>
</svg>
</figure>

The important property is not the total. It is that **the status code, the
document head, and every settled value leave the server before the slow work
finishes**.

## What a handler changes

Almost nothing. Adopting this means passing pending values where you used to
pass finished ones:

```go
func profile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pw.WriteHTML(w, r, Home(HomeParams{
		Profile:        Profile{Name: "Ada Lovelace", Joined: "2026-02-11"},
		Orders:         pw.Go(ctx, loadOrders),
		Recommendation: pw.Go(ctx, recommend),
	}))
}
```

There is no streaming API to call, no header to set, no flush to schedule, and
no loop to write. `pw.WriteHTML` asks the composed document whether it can open
an await boundary and picks its own path. A page without one keeps the ordinary
buffered response and its `Content-Length`.

Whether a response streams is therefore a property of the templates it composed,
not a decision every handler repeats.

## Creating a pending value

`pw.Go` starts work in its own goroutine and hands back a handle:

```go
func loadOrders(ctx context.Context) ([]Order, error) {
	return store.Orders(ctx, customerID)
}

orders := pw.Go(ctx, loadOrders)
```

The context you pass bounds the work and stays yours to cancel. A render bounds
only how long it is willing to wait for it.

| Constructor | Use |
| --- | --- |
| `pw.Go(ctx, work)` | start the work now, in its own goroutine |
| `pw.Resolved(v)` | a value you already have, and tests |
| `pw.Failed(err)` | a failure you already know about |

Three properties are worth knowing.

**A handle settles once and stays readable.** A layout and the page inside it may
hold the same value: both boundaries see the same result, and the work behind it
runs once.

**There is no channel constructor.** A service that already returns a channel is
adopted by receiving from it inside the `pw.Go` closure, so every handle belongs
to a goroutine the framework started — and a panic in one becomes that handle's
error instead of the process's exit.

**Starting early is the point.** The work begins where you call `pw.Go`, so it
can overlap request parsing, authorization, and the rendering of everything
above it.

## Declaring it in a template

Mark the parameter `async` and read it inside an `await` block:

```html
package handlers

type Order {
  id: string
  total: string
}

export component Home(profile: Profile, orders: async Order[]): html {
<h1>{profile.name}</h1>

{await list = orders}
  <ul>{for order in list}<li>{order.id} — {order.total}</li>{/for}</ul>
{fallback}
  <p class="pending">Loading orders…</p>
{/await}
}
```

`async T` is a prefix modifier on any parameter or record field, and it becomes
`pw.Pending[T]` in the generated params struct. It is not callable, and the one
place it may be read is an `await` binding.

The modifier covers the whole type: `async Order[]` is a single pending slice,
not a slice of pending values. When each row has to arrive on its own, give the
row type its own `async` field and await it inside the loop.

A record may carry settled and pending members together — which is what lets the
example above render `profile.name` immediately while the orders are still in
flight.

### The three clauses

```html
{await user = LoadUser(id), posts = LoadPosts(id)}
  ...primary subtree...
{fallback}
  ...committed first, before anything is known...
{recover err}
  ...rendered instead when the bindings fail...
{/await}
```

- Bindings after `await` **start together**. Two slow calls in one block take as
  long as the slower one, not their sum.
- `fallback` is **required**. It is what commits to the response first, so a slow
  dependency never delays the rest of the page.
- `recover` is **optional** and binds a safe error value with `code`, `message`,
  `retryable`, and `timeout` fields.

Bindings are visible only in the primary subtree and the error name only in
`recover`, so no clause can read a value that does not exist when it renders.

A `<slot>` may not appear inside an `await` block — the fallback and the
replacement would both render it. This is also why boundaries from a document, a
layout, and a page are siblings rather than nested: they all start during the
first pass and settle concurrently.

## When something fails

Whether a boundary declares `recover` decides what a failure costs.

**With a `recover` clause**, the failure is contained. That clause renders in
place of its own section and the rest of the page is untouched. The response
stays 200, which is honest — most of it worked.

**Without one**, the page is given up on. The template said what to show while
waiting and what to show on success, and nothing about failure; leaving the
fallback in place would make the page claim forever that it is still loading.
The framework replaces everything below the document shell with an error page.

Register that page once:

```go
pw.RegisterHTMLErrorPage(func(problem pw.Problem) pw.HTMLFragment {
	return Error500(Error500Params{Title: problem.Title})
})
```

It receives the mapped problem, never the original error, so a template cannot
print a cause the server meant to keep. Without a registered resolver a minimal
built-in page is used, so the escalation never depends on application setup.

### Errors stay server-side

A `recover` subtree never sees a raw Go error. By default a failure becomes
`code: "internal"` with no message and a timeout becomes `code: "timeout"`. To
publish something more specific, give the error its own safe projection:

```go
func (e UpstreamError) PublicError() pw.AsyncError {
	return pw.AsyncError{Code: "upstream", Message: "Please try again.", Retryable: true}
}
```

Either way the original reaches the log, with the boundary that produced it:

```
ERROR await boundary failed with no recover clause boundary=tb-1 error="order service returned 503"
```

### Why an unhandled failure is still 200

Because the status left with the shell, long before the failure was known. This
is the honest cost of streaming, and it is worth knowing before you rely on
status codes for monitoring these pages.

Turn streaming off and the same failure answers with a real **500**: the render
then fails while nothing is committed, so the response can still say so. Only
one of the two paths can tell the truth in its status line, and it does.

## Configuration

```toml
[html]
streaming = true
async_timeout = "3s"
async_concurrency = 0
```

| Key | Meaning |
| --- | --- |
| `streaming` | `false` forces the buffered path even when a page could stream |
| `async_timeout` | bounds one await boundary; `0` leaves the request context as the only deadline |
| `async_concurrency` | bounds simultaneous boundary work per render; `0` is unbounded |

An expired boundary renders `recover` with `code: "timeout"`, or escalates if it
has none. Whether the work itself stops is up to the function: one that takes a
`context.Context` sees the cancellation, and one that does not is abandoned —
it finishes on its own and its result is discarded.

Setting `streaming = false` is the escape hatch for a proxy that buffers
responses. The same templates then render as one buffered response that blocks
until every boundary settles; the page is still correct and still complete.

## What the browser does

The document shell loads one small ES module that swaps each completion into
place:

```html
external RuntimeScriptURL(): url

export component Document(children: html?): html {
<!doctype html>
<html><head>...<script type="module" src={RuntimeScriptURL()}></script></head>
<body><slot /></body></html>
}
```

```go
// templates/templates.go
func RuntimeScriptURL() *url.URL { return &url.URL{Path: pw.RuntimeScriptURL()} }
```

The template calls a function rather than writing a literal path, because that
URL carries a revision derived from the script's own bytes. An upgrade that
changes the runtime changes the URL without anyone editing a template, and the
response can claim `Cache-Control: immutable` honestly.

`pw init` scaffolds both halves, so a new project already has them.

No completion carries inline script, so `script-src 'self'` is enough — no
nonce, no `unsafe-inline`.

A client with JavaScript disabled receives the shell and every fallback, and
nothing replaces them. Treat streamed sections as an enhancement over content
that is already meaningful, not as the only way to reach it.

## Restrictions

- An `async` parameter may be read only inside an `await` binding.
- An `await` block requires a `fallback` clause.
- A `<slot>` may not appear inside an `await` block.
- A `@cache` component cannot declare an `async` parameter, or reach a record
  with one: stored bytes stand in for a fresh render, and a pending value
  belongs to the one request that started it.

Each of these is a generation error, so they surface from `pw generate` rather
than at request time.

## A complete example

`examples/async_render` in the repository links one page per behaviour —
success, contained failure, and unhandled failure — so each path is reachable on
purpose rather than by luck.
