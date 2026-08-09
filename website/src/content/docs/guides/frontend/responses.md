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
it, the same buffered body is encoded on the way out.

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

Two consequences follow from having no document around the markup.

A fragment never streams. An await boundary settles in place, so the response
carries no placeholder for a client runtime to replace, and no boundary id that
could collide with one still pending in the page it lands in.

A template that contributes to the document head is rejected with a 500 rather
than losing those contributions silently. A scoped style block belongs in the
head of the page that is already loaded, so declare it in a component that page
renders, or in a shared stylesheet.

Failures answer with `application/problem+json` and their real status rather
than with an HTML error page, because an error document swapped into one region
would replace that region with a whole page. htmx and similar libraries do not
swap a non-2xx response, so the status is the signal they already act on.

`examples/htmx_fragment` is a complete application on this surface: one route
answers with a document, and the filter, the create form, and the delete button
each answer with the region they re-render. It also shows what to do with a
rejected form, since a swap library ignores the problem response a failed check
would produce.

Template syntax, slots, escaping, and scoped styles are covered in
[Templates](/guides/frontend/templates/). For what to build on top of this
surface — dialogs the server fills, toasts, and where a swap stops being the
cheapest answer — see [Fragments and islands](/guides/interactivity/fragments/).

### Cache policy

Every HTML response says whether a shared cache may hold it, and the answer
defaults to no:

```
Cache-Control: private, no-store
```

The answer comes from the templates rather than from the request, and it has to.
`Cache-Control` is on the wire before the first body byte, while a per-reader
component four levels down renders long after that. A signal computed during the
render would therefore exist only on the buffered branch, and a page's cache
policy would end up depending on whether streaming happened to be on.

So the chain is asked before anything renders, and a chain where nothing declared
a scope reports private. That is the answer a login-gated application gets
without writing a line. Declaring the shared answer takes one annotation on the
document shell, because a shell wraps everything below it:

```html
@cache(scope: "public")
export component Document(children: html?): html { … }
```

A shared page then receives no `Cache-Control` from the framework at all.
Freshness is a deployment's decision, and a header naming no lifetime would
either invite heuristic caching or invent a lifetime nobody asked for, so the
framework stops asserting rather than asserting something weaker. Set the
lifetime at your CDN or in a middleware of your own.

On the private side the directive is `no-store` rather than `no-cache`, because a
document carries no entity tag. There is no conditional request to protect, and
`no-store` is what keeps a signed-in page off the disk of a shared machine.

The responses that are not documents keep the policy each one's shape requires. A
navigation delta and a live delivery are `no-store`. A redraw is
`private, no-cache`, which preserves the conditional request its entity tag
exists for. A sequence — the static half of a fragment, derived from the template
rather than from the reader — is `public, max-age=31536000, immutable`.

Know this before putting a CDN in front of a public site: nothing is shared until
a shell declares it, so a marketing page passes straight through the edge until
you write the annotation. That is the intended direction rather than an
oversight. Forgetting the annotation costs a cache miss; the mistake it prevents
costs a reader somebody else's account page.

[`@cache`](/reference/template-syntax/#cache) covers the annotation itself,
including what a private scope does to a component's cache key.

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

A response that arrives over time — tokens, log lines, queue events — is written
with `pw.NewStream[T]` instead:

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

The client chooses between Server-Sent Events, NDJSON, and a JSON array, and the
handler above serves all three unchanged. [Streams](/guides/frontend/streams/)
covers the negotiation, the framing, and what a long-lived response needs from
the rest of the configuration.

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
| `pw.TooManyRequests` | 429 |
| `pw.InternalServerError` | 500 |

Each accepts an `error`, a `string`, another `pw.Problem`, or nothing at all.
Constructor call sites supported by the generator appear in the endpoint's
OpenAPI description. TinyBind v0.5.0 does not yet infer the new 429 helpers, so
their runtime response is complete while generated OpenAPI omits that status.

For an enforced quota, `pw.RateLimited` attaches retry metadata to the same 429
problem:

```go
pw.WriteProblem(w, r, pw.RateLimited(pw.RateLimit{
	Limit:      100,
	Remaining:  0,
	Reset:      resetAt,
	RetryAfter: 30 * time.Second,
}, "request quota exceeded"))
```

The response carries `Retry-After`, `X-RateLimit-Limit`,
`X-RateLimit-Remaining`, and `X-RateLimit-Reset`. The `X-RateLimit-*` names are
compatibility conventions rather than standard HTTP fields; `Retry-After` is
the standard retry signal. A 429 response always carries `Cache-Control:
no-store`, including a bare `pw.TooManyRequests()`.

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

Scaffolded projects carry status templates from `templates/400.pw.html` through
`templates/500.pw.html`, including `templates/429.pw.html`. They are ordinary
components, generated like any other page. The generated error resolver selects
one when `Accept` prefers HTML; the same problem answers as
`application/problem+json` for API clients. Status and response metadata remain
the same on both branches.
