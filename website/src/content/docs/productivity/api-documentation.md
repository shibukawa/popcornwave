---
title: API Documentation
description: An OpenAPI 3.1 document assembled from the code you already wrote, and a browsable UI that reads it.
sidebar:
  order: 4
---

Most OpenAPI documents are written twice: once as handlers, once as a
specification that drifts away from them within a release or two.

Popcorn Wave assembles the document from the code instead. `pw generate` already
reads your route registrations, your `pw.Parse[T]` call sites, your `check` tags,
and your `pw.WriteAPI` calls to write binding code — the same evidence describes
the endpoint. One OpenAPI 3.1 fragment is emitted per package, and the framework
merges them at startup.

Nothing is annotated. There is no separate specification file to keep in sync,
because there is no separate specification.

## Serving it

```toml
[server]
openapi = "/openapi.json"    # unset serves nothing
api_doc = "scalar"           # "scalar", "swagger", or empty to disable
api_doc_path = "/docs"
```

`openapi` names the path the merged document answers on. It has no default: an
endpoint nobody wrote down is an endpoint nobody audits, so an unset key
registers no route. `api_doc` adds a browsable UI over it — Scalar or Swagger UI
— and requires `openapi`; a non-empty `api_doc` without it fails startup with
`server.api_doc requires server.openapi` rather than serving a page that cannot
load its own spec.

`pw init` writes `api_doc = "scalar"` into `config.dev.toml` only. The default is
empty, so the reference stays private until a staging or production config opts
in.

## Who can read it

An API description is a map of your whole surface, so both paths are mounted
beneath the authentication chain. `auth.protection.include` covers them exactly
as it covers an application route:

```toml
[auth.protection]
include = ["/openapi.json", "/docs"]
```

Protection is opt-in, so without a matching pattern they stay public like any
unlisted route. The [health and readiness probes](/guides/deployment/operational-endpoints/)
are the two endpoints that can never be protected — nothing authenticates above
them, which is what a liveness check needs.

The UI page is a few hundred bytes of HTML that loads the interface from a
public CDN, so the binary stays small. The browser then has to reach that CDN,
and it has to run the inline script that starts the interface. A
Content-Security-Policy written for your own pages blocks both, and the page
renders blank.

The endpoint answers that itself. When a policy is present on the response, it
is replaced with the one this page actually needs:

```
script-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; style-src 'self' https://cdn.jsdelivr.net 'unsafe-inline'; img-src 'self' data:; font-src 'self' https://cdn.jsdelivr.net data:; connect-src 'self'
```

The substitution is per response, so the CDN host and the inline allowances
never leave this page — every other route keeps
`security.headers.content_security_policy` exactly as you wrote it. Widening
that key instead would have carried both into every response the application
sends.

An application that configures no policy still receives no header here. One that
uses `content_security_policy_report_only` has that header replaced instead, so
the documentation page stops filling the report with violations you cannot act
on.

## What the document already knows

Given a handler like this:

```go
type listItemsInput struct {
	Page  int    `query:"page" check:"min=1" default:"1"`
	Sort  string `query:"sort" enum:"asc,desc" default:"asc"`
	Owner string `query:"owner" check:"email"`
}

func listItems(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[listItemsInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	// ...
	pw.WriteAPI(w, r, items)
}
```

the generated fragment carries the path and method from the route registration,
`operationId: listItems`, three query parameters with `minimum`, `enum`,
`default`, and `format: email` on them, a `200` response referencing the item
schema, and a `400` referencing `ProblemDetails` — because the handler passes a
parse failure to `pw.WriteProblem`.

Responses come from what the handler actually calls:

| In the handler | In the document |
| --- | --- |
| `pw.WriteAPI` | `200` with the response schema |
| `pw.WriteStatus` | one response per static status it is called with |
| `pw.NewStream[T]` | `text/event-stream`, `application/x-ndjson`, and `application/json` |
| `pw.BadRequest`, `NotFound`, `Conflict`, … | that status, as `application/problem+json` |
| any `check` rule on the request | `400`, even without an explicit error call |

Path parameters are `required` automatically. Body fields become a request body
accepting JSON, form-encoded, and multipart — the same three formats the binding
accepts.

## Making it better

The document is generated, but how good it reads is up to you. Three habits do
most of the work.

### Write the godoc you should be writing anyway

Handler doc comments become the operation text. The **first sentence** is the
`summary`, the **rest** is the `description`:

```go
// List the catalogue. Results are paginated and ordered by name unless the
// caller asks otherwise.
//
// The owner filter is applied before pagination.
func listItems(w http.ResponseWriter, r *http.Request) {
```

produces

```json
"summary": "List the catalogue.",
"description": "Results are paginated and ordered by name unless the caller asks otherwise.\n\nThe owner filter is applied before pagination."
```

The text is carried verbatim — the generator does not reword it or strip the
`FuncName ...` prefix Go convention starts with. Writing `// List the catalogue.`
rather than `// listItems lists the catalogue.` gives a summary that reads well
in the UI and still reads fine in `go doc`.

A godoc `Deprecated:` paragraph sets `deprecated: true` on the operation, so the
UI strikes it through:

```go
// Legacy is the previous listing endpoint.
//
// Deprecated: use listItems instead.
```

### Document the fields, not just the endpoint

Doc comments on struct fields become parameter and property descriptions, and a
comment on the type becomes the schema description:

```go
// item is one catalogue entry.
type item struct {
	// ID is the stable identifier.
	ID int `json:"id"`
	// Name is shown to the reader.
	Name string `json:"name"`
}
```

Both the request and the response types are read this way, so a comment written
for the next Go developer also documents the API for its consumers.

### Declare the constraints, and they document themselves

Every constraint maps to a JSON Schema keyword, so validation you had to write
anyway becomes the machine-readable part of the contract:

| Declaration | Schema |
| --- | --- |
| `check:"required"` | listed in `required` |
| `check:"min"` / `check:"max"` | `minimum` / `maximum` |
| `check:"minlen"` / `check:"maxlen"` | `minLength` / `maxLength` |
| `check:"len"` | both `minLength` and `maxLength` |
| `check:"pattern=…"` | `pattern` |
| `check:"email"`, `uuid`, `date`, `time`, `datetime` | `format` |
| `enum:"a,b"` | `enum` |
| `default:"…"` | `default` |

The last two are tags of their own rather than `check` rules; writing them
inside `check` is an error. See [Handlers](/guides/frontend/handlers/#defaults-and-enumerations).

## Naming the API

Each fragment defaults to `"<package> API"` at version `0.0.0`, and the assembled
document falls back to `Application API`. Set it once, before serving:

```go
func main() {
	if err := pw.SetOpenAPIInfo(pw.OpenAPIInfo{
		Title:   "Catalogue API",
		Version: "1.4.0",
	}); err != nil {
		log.Fatal(err)
	}
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

Both fields are required. Calling it twice with the same value is harmless;
calling it with a conflicting second value is an error, so two packages cannot
quietly disagree about what the API is called.

## Reaching the document yourself

| Call | Use |
| --- | --- |
| `pw.AssembleOpenAPI()` | the merged document as JSON bytes |
| `pw.OpenAPIJSON(w, r)` | the handler behind `openapi.path` |
| `pw.ScalarUI(specURL)` | a Scalar page for any spec URL |
| `pw.SwaggerUI(specURL)` | a Swagger UI page for any spec URL |

These are what an application that mounts its own routes uses, and what a build
step can call to write the document to a file for client generation:

```go
doc, err := pw.AssembleOpenAPI()
```

Fragments merge by path and by component name. Two packages that define
different schemas under the same name are renamed apart rather than silently
overwriting each other; two fragments registering the same package ID is an
error.

## What it does not cover

Only routes the generator can see statically are described — a handler mounted
through a variable the generator cannot follow will not appear. Server-rendered
HTML endpoints are not described either; the document covers the JSON and
streaming surface, which is what a client generator can use.

See [Handlers](/guides/frontend/handlers/) for the binding and validation tags,
[Responses](/guides/frontend/responses/) for the write calls, and
[`pw generate`](/pw/project/generate/) for when the fragments are produced.
