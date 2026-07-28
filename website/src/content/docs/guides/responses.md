---
title: Responses
description: Returning HTML, JSON, streams, and RFC 9457 errors.
sidebar:
  order: 2
---

A handler may produce HTML, JSON, a stream, or an error, but the same constraint
applies to all four: status and headers must be settled before the body begins.
The response helpers enforce that boundary while preserving the wire format each
case needs.

## HTML

```go
pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name}))
```

`Home` and `HomeParams` are generated from `handlers/home.pw.html`. The handler
passes a **leaf fragment**; `WriteHTML` renders it inside the application's
registered document shell, so the handler never names, imports, or constructs
the document.

The whole chain is rendered into a buffer and validated **before** anything is
committed. A template failure can therefore become a clean 500 instead of
leaving a half-written page. If compression is enabled and the client accepts
it, the same buffered body is zstd-encoded on the way out.

For explicit control over the wrapper chain — one page inside a different shell
— use `pw.WriteHTMLChain`:

```go
pw.WriteHTMLChain(w, r,
	[]pw.HTMLWrapper{templates.BindPrintDocument(templates.PrintDocumentParams{})},
	Invoice(InvoiceParams{ID: input.ID}),
)
```

### Fragments

An htmx-style interaction replaces one region of a page that already exists, so
it needs that region rather than a document. `pw.WriteHTMLFragment` renders one
template and nothing else:

```go
pw.WriteHTMLFragment(w, r, Row(RowParams{Item: item}))
```

No document shell, no wrapper chain, no merged head, no framing. The body is
exactly what the template wrote, ready for the swap library to insert.

Two consequences follow from having no document around the markup. A fragment
never streams: an await boundary settles in place, so the response carries no
placeholder for a client runtime to replace and no boundary id that could
collide with one still pending in the page it lands in. And a template that
contributes to the document head is rejected with a 500 instead of losing those
contributions silently — a scoped style block belongs in the head of the page
that is already loaded, so declare it in a component that page renders, or in a
shared stylesheet.

Failures answer with `application/problem+json` and their real status rather
than with an HTML error page, because an error document swapped into one region
would replace that region with a whole page. htmx and similar libraries do not
swap a non-2xx response, so the status is the signal they already act on.

Template syntax, slots, escaping, and scoped styles are covered in
[Templates](/guides/templates/).

## JSON

```go
pw.WriteAPI(w, r, user)
```

The call site generates an encoder for the response type, removing runtime
reflection. The encoder uses the name portion of `json` tags, but it does not
interpret `omitempty` or exclusion directives. The declared type must therefore
match the shape you intend to send.

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

Format selection begins with the `?stream=` query parameter, then considers
`Accept`, then User-Agent heuristics, and finally falls back to NDJSON. If an
`Accept` header rules out every supported type, the stream starts with a `406
Not Acceptable` problem response. Every later `Send` returns that same error
instead of writing a contradictory body.

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

That mapping lets a handler forward a service error without adding a second
translation layer:

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

The current boundary matters here. `pw.WriteProblem` always answers with
`application/problem+json`, while `pw.WriteHTML` accepts no status code and
therefore answers 200. Applications that want one of these templates under a
4xx or 5xx status must wire that path themselves.
