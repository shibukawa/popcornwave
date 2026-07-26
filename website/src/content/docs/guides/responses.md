---
title: Responses
description: Returning HTML, JSON, streams, and RFC 9457 errors.
sidebar:
  order: 2
---

A handler ends by writing exactly one response. There are four ways to do it,
and all four choose status and headers before the body is written.

## HTML

```go
pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name}))
```

`Home` and `HomeParams` are generated from `handlers/home.pw.html`. The handler
passes a **leaf fragment**; `WriteHTML` renders it inside the application's
registered document shell, so the handler never names, imports, or constructs
the document.

The whole chain is rendered into a buffer and validated **before** anything is
committed, so a template failure becomes a clean 500 rather than a half-written
page. When compression is enabled and the client accepts it, the buffered body
is zstd-encoded on the way out.

For explicit control over the wrapper chain — one page inside a different shell
— use `pw.WriteHTMLChain`:

```go
pw.WriteHTMLChain(w, r,
	[]pw.HTMLWrapper{templates.BindPrintDocument(templates.PrintDocumentParams{})},
	Invoice(InvoiceParams{ID: input.ID}),
)
```

Template syntax, slots, escaping, and scoped styles are covered in
[Templates](/guides/templates/).

## JSON

```go
pw.WriteAPI(w, r, user)
```

The encoder for the response type is generated from the call site, so there is
no reflection at run time. It uses the name portion of `json` tags but does not
interpret `omitempty` or exclusion directives — declare the shape you want to
send.

The status is 200. `pw.WriteAPI` call sites also feed the generated OpenAPI
document, so a JSON endpoint is described without a separate annotation pass.

## Streams

```go
func events(w http.ResponseWriter, r *http.Request) {
	stream := pw.NewStream[ChatEvent](w, r)
	defer stream.Close()

	for event := range source {
		if err := stream.Send(event); err != nil {
			return
		}
	}
}
```

`pw.NewStream[T]` negotiates the wire format from the request and starts the
response. `Send` writes one value; `Close` finalises the response — which
matters for the JSON-array format, whose closing bracket is written there.

| Format | Media type |
| --- | --- |
| Server-Sent Events | `text/event-stream` |
| NDJSON | `application/x-ndjson`, `application/ndjson`, `application/jsonl` |
| JSON array | `application/json` |

Selection order is the `?stream=` query parameter, then `Accept`, then
User-Agent heuristics, then NDJSON as the fallback. An `Accept` header that
asks for none of the supported types gets a `406 Not Acceptable` problem
response, and every subsequent `Send` returns that error rather than writing.

Note that `server.write_timeout` defaults to `0s` precisely so long-lived
streams are not cut off; see [Configuration](/guides/configuration/).

## Errors

`pw.WriteProblem` writes an **RFC 9457 Problem Details** response as
`application/problem+json`:

```go
pw.WriteProblem(w, r, pw.NotFound("no such user"))
```

```json
{
  "type": "about:blank",
  "title": "Not Found",
  "status": 404,
  "detail": "no such user",
  "code": "not_found"
}
```

### Constructors

| Constructor | Status |
| --- | --- |
| `pw.BadRequest` | 400 |
| `pw.Forbidden` | 403 |
| `pw.NotFound` | 404 |
| `pw.InternalServerError` | 500 |

Each accepts an `error`, a `string`, another `pw.Problem`, or nothing at all.
These call sites are also read by the generator, so the error responses an
endpoint can produce appear in its OpenAPI description.

For a status without a constructor, build the value directly:

```go
pw.WriteProblem(w, r, pw.Problem{
	Status:  http.StatusConflict,
	Title:   "Conflict",
	Code:    "already_registered",
	Message: "that email is already registered",
})
```

### Passing errors through

`pw.WriteProblem` takes any `error` and maps it:

- a `pw.Problem` (including one wrapped with `%w`) is used as-is;
- a binding or validation error keeps its own status and field detail;
- anything else becomes a 500.

So a handler can forward what a service returned without translating it:

```go
if err := service.Register(r.Context(), input); err != nil {
	pw.WriteProblem(w, r, err)
	return
}
```

### Two safety behaviours

**5xx details never leak.** A status of 500 or above is logged in full with the
request-scoped logger, then reported to the client as `internal error` with code
`internal`.

**A committed response is never corrupted.** If the body has already started,
`WriteProblem` logs the error instead of appending a second, contradictory
payload.

### HTML error pages

Scaffolded projects carry `templates/400.pw.html`, `404.pw.html`, and
`500.pw.html`. They are ordinary components, generated like any other page.

Be aware of the current boundary: `pw.WriteProblem` is the framework's error
path and always answers with `application/problem+json`, and `pw.WriteHTML`
takes no status code, so it always answers 200. Rendering one of these
templates under a 4xx or 5xx status is something the application wires up
itself today.
