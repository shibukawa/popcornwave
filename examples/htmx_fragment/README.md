# htmx Fragments

A task board where every interaction re-renders one region of a page that
already exists, and the server answers with that region's markup and nothing
else.

```bash
go run ./cmd/htmx_fragment
```

Then open http://localhost:8080. The board is held in memory, so restarting the
process restores the seeded rows.

## The one call that differs

Compare the two response calls in
[handlers/tasks_handler.go](handlers/tasks_handler.go):

```go
pw.WriteHTML(w, r, Home(HomeParams{…}))                    // GET /
pw.WriteHTMLFragment(w, r, TaskList(TaskListParams{…}))    // GET /tasks
```

Everything around them is the same handler shape: `pw.Parse` for the request,
the generated `Params` struct for the template, no header set by hand.
`WriteHTMLFragment` resolves no document shell, merges no head, composes no
wrapper chain, and writes no boundary framing. The body is what the template
wrote, so the swap library can insert it as it stands.

That is the whole integration. htmx is configured entirely in the templates —
`hx-post`, `hx-target`, `hx-swap` — and the framework contributes markup and a
status code. There is no envelope to parse and no client library on the Go side.

## One definition of the markup

`TaskPanel` and `TaskList` in [handlers/tasks.pw.html](handlers/tasks.pw.html)
are called twice: by `Home` for the first paint, and by the partial handlers
for every swap after it.

```text
<TaskPanel form={form} tasks={tasks} emptyLabel={emptyLabel} note="" />
```

The page and the partial cannot disagree about what a task row looks like,
because there is one component and the compiler type-checks both call sites.

Three regions demonstrate different targets:

| Interaction | Route | Target |
| --- | --- | --- |
| Filter box | `GET /tasks` | `#task-list` |
| Add form | `POST /tasks` | `#task-panel` |
| Remove button | `DELETE /tasks/{id}` | `#task-list` |

The filter box lives outside the panel, and the form and the remove buttons
send it along with `hx-include="#filter"`, so a write returns the list the
operator is actually looking at rather than the unfiltered one.

## The status contract

htmx swaps a 2xx response and ignores everything else. That single rule decides
how each failure is answered here.

**A rejected form comes back as HTML with a 200.** The checks stay declared on
the request struct:

```go
type createInput struct {
	Title string `payload:"title" check:"required,maxlen=60"`
	Owner string `payload:"owner" check:"required,maxlen=24"`
	…
}
```

A typo is expected traffic rather than a malformed request, so `createTask`
does not pass that failure to `pw.WriteProblem`: a problem document is not
swapped, and the page would sit there showing nothing about why nothing
happened. It reads the field errors out of the `pw.Parse` error and re-renders
the panel with them — with the operator's own text still in the fields, which
comes from the already-parsed request, since `pw.Parse` answers with the zero
value when a check fails.

**A stale click stays a 404.** Removing a task that is already gone answers
`application/problem+json` with a real status. htmx leaves the list alone,
which is the honest outcome, and the response is legible to anything else that
calls the same route.

## Async inside a fragment

`GET /tasks/summary` hands the render a value that has not arrived yet, exactly
as a page handler would:

```go
pw.WriteHTMLFragment(w, r, TaskSummary(TaskSummaryParams{
	Summary: pw.Go(r.Context(), summarize),
}))
```

The delivery is what differs. A fragment response is always buffered, so the
`await` boundary settles on the server and the body arrives finished:

```bash
curl -s http://localhost:8080/tasks/summary
```

```html
<div class="summary"> <p>3 tasks · 1 of them high priority · counted in 600ms</p> </div>
```

No placeholder, no boundary id, and nothing for a client runtime to apply —
which is the point, because the document this lands in may already hold a
boundary of its own, and a swap library holds the body rather than letting the
parser consume it as it arrives. Note what follows: the `fallback` the template
declares never reaches the browser on this path. The waiting state on the page
belongs to htmx (`hx-indicator`, the `htmx-request` class), not to the
template.

`html.streaming` in [config.dev.toml](config.dev.toml) changes nothing here.
It selects how a *page* is delivered; this path has no streaming branch to pick.

## What a fragment cannot deliver

`GET /tasks/broken` renders a component that declares a scoped `style` block:

```text
export component StyledBadge(label: string): html {
<head>
<style>
.demo-badge { color: crimson; font-weight: 700 }
</style>
</head>
<p class="demo-badge">{label}</p>
}
```

A scoped style is hoisted into the document head, and a fragment response has
no head to receive it. Dropping it silently would swap in an unstyled region
with nothing in any log, and inlining it would re-emit the tags on every swap
with nothing deduplicating them — so the framework answers 500 with a problem
document before rendering anything:

```json
{"type":"about:blank","title":"Internal Server Error","status":500,…}
```

Click the button on the page and watch the target: it keeps what it already
had, because htmx does not swap a 500. The server log carries the reason.

Styles that a swapped region needs belong to the page that is already loaded:
declare them in a component the page renders, or in a shared stylesheet as
[public/app.css](public/app.css) does here.

## Where htmx comes from

[templates/document.pw.html](templates/document.pw.html) loads it from a CDN,
pinned by version and by Subresource Integrity:

```html
<script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.10/dist/htmx.min.js"
        integrity="sha384-H5SrcfygHmAuTDZphMHqBJLc3FhssKjG7w/CeCpFReSfwBWDTKpkzPP8c+cLsK+V"
        crossorigin="anonymous"></script>
```

So the example needs network access on first load, and a page under a
`script-src 'self'` policy would have to name the CDN as well. Serving htmx
yourself removes both, at the cost of a vendored file in the repository.

### Serving htmx from `public/`

Download the same pinned version into the example's `public` directory:

```bash
curl -fsSL https://cdn.jsdelivr.net/npm/htmx.org@2.0.10/dist/htmx.min.js -o public/htmx.min.js
```

Check the bytes against the integrity value the tag above pins, rather than
trusting the transfer. The two strings must match:

```bash
openssl dgst -sha384 -binary public/htmx.min.js | openssl base64 -A
```

Then point the document at the local copy — same file, one origin, so the
integrity and `crossorigin` attributes have nothing left to do:

```html
<script src="/public/htmx.min.js"></script>
```

That is the whole change. [public.go](public.go) already embeds the directory
with `//go:embed all:public` and registers it during `init`, so a new file under
`public/` is served without touching any Go code, and `/public` is the default
`server.public.mount`.

Two things follow from the build, both of them automatic:

- `pw build` walks `public/`, writes a deterministic `htmx.min.js.zstd` beside
  the source, and only then compiles. A client that accepts `zstd` gets the
  precompressed representation with the right `Content-Type`; anything else
  gets the original bytes. `.gitignore` already excludes `public/**/*.zstd`,
  because the sidecar is a build output.
- `pw dev` serves the directory from disk instead, so replacing the file during
  an upgrade takes effect on reload rather than at the next build.

Commit `public/htmx.min.js` itself. The embed captures whatever is in the
directory when the binary is built, and a build from a checkout that lacks the
file still succeeds — it just serves a 404 for the script and leaves every
`hx-` attribute inert. Upgrading means repeating the two commands above with
the new version; nothing in the templates or handlers names it.

Nothing else about the application changes either way: the framework never names
the swap library, and no route knows which one made the request.

## Tests

[handlers/tasks_handler_test.go](handlers/tasks_handler_test.go) drives the mux
directly, so path wildcards bind the way they do in a browser. The assertions
are the properties this example exists to show: the page carries a document,
the partials carry no document, a rejected form comes back as HTML, a stale
delete is a 404, the summary arrives settled, and a head contribution is
refused.

```bash
go test ./...
```
