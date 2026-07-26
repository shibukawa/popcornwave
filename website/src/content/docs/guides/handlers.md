---
title: Handlers
description: Routing and request binding — struct tags, validation, and JSON, form, and multipart bodies.
sidebar:
  order: 1
---

`github.com/shibukawa/popcornwave/pw` is the stable application-facing API.
Everything on this page comes from it.

## Routing

```go
package handlers

import "github.com/shibukawa/popcornwave/pw"

var mux = pw.NewServeMux()

func Handlers() *pw.ServeMux { return mux }
```

On ordinary Go builds `pw.ServeMux` **is** `net/http`'s `ServeMux` — a type
alias, not a wrapper — so patterns, wildcards, and precedence are exactly the
standard library's. TinyGo gets a separate implementation with the same
semantics, because it has no standard mux.

Each handler file registers itself in `init`, so adding a route means adding a
file rather than editing a central table:

```go
func init() {
	mux.HandleFunc("GET /", home)
	mux.HandleFunc("GET /users/{id}", showUser)
	mux.HandleFunc("POST /users", createUser)
}
```

For splitting routes across packages as an application grows, see
[Project structure](/guides/project-structure/).

## Binding a request

`pw.Parse[T]` fills a struct from the request. Generation reads the call site,
so the binding code for `T` is written ahead of time rather than reflected at
run time.

```go
type showUserInput struct {
	ID   int    `path:"id"`
	Sort string `query:"sort" default:"name"`
}

func showUser(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[showUserInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	// ...
}
```

### Source tags

| Tag | Source |
| --- | --- |
| `input:"name"` *(or no tag)* | query string, falling back to the body |
| `query:"name"` | query string only |
| `payload:"name"` | request body only |
| `path:"id"` | path wildcard, from a pattern like `/users/{id}` |
| `header:"Authorization"` | request header |
| `cookie:"session"` | request cookie |
| `method:"method"` | the HTTP method |

Without an explicit wire name, the field name is lowerCamelCased —
`DisplayName` becomes `displayName`.

`input` is the forgiving default: a query parameter wins, and the body is only
read when the query does not supply the value. Use `query` or `payload` when
you want exactly one source.

### Request bodies

One request struct accepts all three body formats, so the client chooses:

- `application/json`
- `application/x-www-form-urlencoded`
- `multipart/form-data`

An ordinary HTML form post and a JSON API call can therefore share a handler:

```go
type createUserInput struct {
	Name  string `payload:"name" check:"required,maxlen=40"`
	Email string `payload:"email" check:"required,email"`
}
```

### File uploads

Multipart file fields use `httpbind.File`:

```go
import "github.com/shibukawa/tinybind-go/httpbind"

type uploadInput struct {
	Title string        `payload:"title" check:"required"`
	Image httpbind.File `payload:"image" check:"required"`
}
```

`File` exposes `Filename`, `ContentType`, `Size`, and `Content`. The multipart
body limit defaults to 1 MiB and is changed with
`httpbind.SetMaxMultipartBodyBytes`. Note that the framework's own
`server.max_request_body` (10 MiB by default) applies first — see
[Configuration](/guides/configuration/).

### Undeclared fields

`payload:"*"` collects everything you did not declare:

```go
type eventInput struct {
	Type   string         `payload:"type"`
	Extras map[string]any `payload:"*"`
}
```

Use `map[string]json.RawMessage` to keep the raw JSON instead of decoded values.

## Validation

The `check` tag declares constraints. They are compiled during generation, not
interpreted per request.

| Rule | Applies to | Example |
| --- | --- | --- |
| `required` | any | rejects empty strings, missing values, empty files |
| `default=value` | scalars | applied when the value is absent |
| `min` / `max` | numbers | `check:"min=1,max=100"` |
| `minlen` / `maxlen` / `len` | strings | `check:"maxlen=40"` |
| `enum=a\|b\|c` | scalars | `check:"enum=asc\|desc"` |
| `pattern=...` | strings | regular expression |
| `email` | strings | RFC format |
| `uuid` | strings | UUID format |
| `date` | strings | `YYYY-MM-DD` |
| `time` | strings | `HH:MM:SS` |
| `datetime` | strings | RFC 3339 |

Separate rules with commas. If a `pattern` contains a comma, put it last.

```go
type listInput struct {
	Page int    `query:"page" check:"min=1" default:"1"`
	Sort string `query:"sort" check:"enum=asc|desc" default:"asc"`
}
```

A failed check makes `pw.Parse` return an error carrying the offending field.
Passing it to `pw.WriteProblem` produces a 400 with field-level detail — see
[Responses](/guides/responses/).

## Request-scoped accessors

| Call | Returns |
| --- | --- |
| `pw.Logger(ctx)` | request-scoped `*slog.Logger` |
| `pw.Config[T](ctx)` | a registered configuration struct |
| `pw.DB(ctx)` | `(*sql.DB, bool)` |
| `pw.Transaction(ctx, fn)` | runs `fn` inside a transaction |

```go
func createUser(w http.ResponseWriter, r *http.Request) {
	input, err := pw.Parse[createUserInput](r)
	if err != nil {
		pw.WriteProblem(w, r, pw.BadRequest(err))
		return
	}
	err = pw.Transaction(r.Context(), func(ctx context.Context) error {
		if _, err := queries.InsertUser(ctx, input.Name, input.Email); err != nil {
			return err
		}
		return queries.RecordAudit(ctx, "user.created")
	})
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}
```

Generated query functions inside the callback pick the transaction up from the
context. See [Queries](/guides/queries/).

## The lifecycle

```go
func main() {
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

`pw.Run` parses configuration, handles application flags such as
`--generate-config`, validates the configured runtime, initialises the database
pool, checks for collisions between your routes and the operational endpoints,
builds the middleware stack, serves, shuts down gracefully on `SIGINT` or
`SIGTERM`, and closes registered resources in reverse order.

When you need the wrapped handler without owning the server — behind another
listener, or in a test — `pw.Middlewares(handler, options...)` performs the same
initialisation and returns the same stack as a plain `http.Handler`.

`pw.WithPublicFS(fsys)` supplies the embedded public tree explicitly;
scaffolded projects instead register it from `public.go`.
