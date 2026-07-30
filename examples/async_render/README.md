# Async Render

A page whose slow sections stream in after the rest of it has already been
delivered, over one HTTP response.

```bash
go run ./cmd/async_render
```

Then open http://localhost:8080, which links one page per behaviour.

## What the handler does differently

Nothing, almost. Compare [handlers/home_handler.go](handlers/home_handler.go)
with any other handler: it builds a params struct and calls `pw.WriteHTML`. The
only difference is that two of those fields hold `pw.Go(...)` instead of a value
it waited for.

```go
pw.WriteHTML(w, r, Home(HomeParams{
	Profile:        Profile{Name: "Ada Lovelace", Joined: "2026-02-11"},
	Orders:         pw.Go(ctx, loadOrders),
	Recommendation: pw.Go(ctx, recommend),
}))
```

(The real handler wraps each call to pass the `fail` query parameter through.)

There is no streaming API to call, no header to set, and no loop to write. The
framework notices that the composed chain can open an await boundary and streams
on its own; a page without one keeps its buffered response and its
`Content-Length`.

## What the template declares

[handlers/home.pw.html](handlers/home.pw.html) marks the two parameters `async`
and reads each one inside an `await` block:

```text
export component Home(profile: Profile, orders: async Order[], recommendation: async string): html {
```

`fallback` is required — it is what commits to the response first, so a slow
dependency never delays the rest of the page.

## The two failure paths

`recover` is optional, and whether a boundary declares one is the whole
difference between the two failure pages the index links.

**With a recover clause** (`/profile?fail=recommendation`) the failure is
contained: that clause renders in place of its own section and the rest of the
page is untouched. The response stays 200, which is honest — most of it worked.

**Without one** (`/profile?fail=orders`) the page is given up on. The template
said what to show while waiting and what to show on success, and nothing about
failure, so leaving its fallback in place would make the page claim forever that
it is still loading. The framework replaces everything below the shell with
[templates/500.pw.html](templates/500.pw.html), registered through
`pw.RegisterHTMLErrorPage` in [templates/errors.go](templates/errors.go).

Either way the Go error never reaches the browser. A recover clause sees a safe
`pw.AsyncError`; an error page sees the mapped problem. The original text goes
to the log, with the boundary that produced it:

```
ERROR await boundary failed with no recover clause boundary=tb-1 error="order service returned 503"
```

## Why the unhandled case is still 200

Because the status left with the shell, long before the failure was known. That
is the honest cost of streaming, and it is worth knowing before you rely on
status codes for monitoring here.

Set `streaming = false` under `[html]` in `config.dev.toml` and reload that same
URL: the render now fails while nothing is committed, so it answers with a real
**500** carrying the same error page. Only one of the two branches can tell the
truth in its status line, and it does.

## Seeing it work

The work takes 900 ms and 1500 ms. Watching the response arrive shows both the
early commit and the overlap:

```bash
curl -sN --raw http://localhost:8080/profile
```

```
 0.02s  shell, profile, and both fallbacks
 0.92s  orders
 1.52s  recommendation
```

Total is 1.5 s rather than 2.4 s, because the two boundaries run concurrently
rather than in the order the template happens to mention them.

## The browser side

The document shell loads one small module that swaps each completion into place:

```text
<script type="module" src={RuntimeScriptURL()}></script>
```

The template calls a function rather than writing a path, because the URL
carries a revision derived from the script's own bytes. That is what lets the
response say `Cache-Control: immutable` honestly, and it means an upgrade that
changes the runtime changes the URL without anyone editing a template.

No completion carries inline script, so `script-src 'self'` is enough — no
nonce, no `unsafe-inline`.

## Turning it off

`config.dev.toml` carries the knobs:

```toml
[html]
streaming = true
async_timeout = "3s"
async_concurrency = 0
```

Set `streaming = false` and the same templates render as one buffered response
that blocks until every boundary settles. The page is still correct and still
complete.

A client with JavaScript disabled sees the streamed version differently: the
shell and every fallback arrive, and nothing ever replaces them, because both
the completions and the error page need the runtime to be applied.
