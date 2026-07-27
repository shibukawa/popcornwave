---
title: Interoperability
description: Which parts are the framework, which are replaceable helpers, and how to use those helpers from another framework.
sidebar:
  order: 1
---

What Popcorn Wave insists on is small: a middleware stack over `net/http` and
the tooling that keeps generation, development, and builds honest. Request
parsing, database queries, HTML, and response writing are **helpers**. Each one
can be dropped in favour of something you prefer, and each one can be taken to
another framework without bringing Popcorn Wave along.

## Why these layers exist at all

Most Go libraries reach for `reflect`, and for good reason: it is what lets a
request bind into any struct, rows map onto any model, and a template walk any
data. The cost is that reflection-heavy code is exactly what **TinyGo** handles
worst — support is partial, and what does compile pays for it in binary size.
For a framework that treats TinyGo as a real target, the usual libraries were
not an option.

So each layer was written as a **code generator** instead. The work reflection
would otherwise do on every request is done once, ahead of time, by
[`pw generate`](/pw/project/generate/): binders for the types you actually
bind, encoders for the types you actually write, scanners for the columns your
SQL actually selects. Nothing in the hot path inspects a type at run time.

That decision has a corollary that matters more than the performance: a
generator only knows what it can read in your sources, so the framework never
took ownership of the handler signature to compensate. A handler stays an
`http.HandlerFunc`, `w` and `r` stay the standard types. Anything the generated
layer does not cover, you can still do by hand — or with a reflect-based
library, if you accept giving up the TinyGo target for that build.

## What is actually the framework

| Part | Role |
| --- | --- |
| `middlewares` — request ID, recovery, body limit, security headers, timeout, access log, assets, OpenTelemetry | the framework |
| `pw.Middlewares` — configuration, startup validation, database pool, extensions, operational endpoints, the assembled stack | the framework |
| `pw.Run` | a wrapper over `pw.Middlewares` and `http.Server` |
| `pw generate`, `pw dev`, `pw build`, `pw migrate`, `pw seed` | the developer experience |
| `pw.Parse[T]` | replaceable helper |
| `.pw.sql` statements | replaceable helper |
| `.pw.html` components | replaceable helper |
| `pw.WriteAPI` / `WriteHTML` / `NewStream` / `WriteProblem` | replaceable helper |

The helpers are how four common jobs get done in very little code, and they are
what feeds the generated OpenAPI document. They are not a contract the runtime
depends on.

## Owning the server

`pw.Run` is a convenience wrapper, not a requirement. All the framework
initialisation lives in `pw.Middlewares`, which hands back a plain
`http.Handler`:

```go
handler, err := pw.Middlewares(handlers.Handlers())
if err != nil {
	log.Fatal(err)
}
log.Fatal(http.ListenAndServe(":8080", handler))
```

That is the whole escape hatch — put the handler behind your own
`http.Server`, another listener, `httptest`, a Lambda adapter, or a host process
that also serves something else.

What `pw.Run` adds on top, and what you then own yourself:

| `pw.Run` also does | If you serve it yourself |
| --- | --- |
| handles `--generate-config` and the other framework flags | they stop working |
| builds `http.Server` from `[server]` — port and the four timeouts | read `pw.Config[pw.ServerConfig](nil)` and apply what you want |
| shuts down on `SIGINT` / `SIGTERM` within `shutdown_timeout` | your own signal handling and `server.Shutdown` |
| closes extension resources in reverse registration order | not exported today; sessions and the database pool are released by process exit instead |

For a test or a short-lived process none of that matters. For a long-running
deployment, the shutdown ordering is the part worth reproducing.

## Replacing a helper inside a Popcorn Wave application

### Request parsing by hand

```go
func createUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	// or: json.NewDecoder(r.Body).Decode(&input)
}
```

Nothing objects. What you give up is what generation was doing for you: the
`check` rules, the field-level problem response, and the endpoint's request
schema in the OpenAPI output. That is a fair trade for one odd endpoint and a
poor one for forty.

### Queries with sqlx, GORM, or plain `database/sql`

The pool is a `*sql.DB` and it is available from the request context:

```go
db, ok := pw.DB(r.Context())
if !ok {
	pw.WriteProblem(w, r, pw.ServiceUnavailable("database unavailable"))
	return
}
users := sqlx.NewDb(db, driver) // or gorm.Open(postgres.New(postgres.Config{Conn: db}))
```

`pw.Transaction` puts its transaction in the context for **generated**
statements, which is why they need no explicit handle. Another library cannot
see it, so reach for the transaction itself when one request has to mix both:

```go
err := pw.Transaction(r.Context(), func(ctx context.Context) error {
	if _, err := queries.InsertUser(ctx, input.Name); err != nil {
		return err
	}
	executor, err := sqlbind.SQLExecutorFromContext(ctx)
	if err != nil {
		return err
	}
	tx, ok := executor.(*sql.Tx)
	if !ok {
		return errors.New("no transaction in context")
	}
	return audit(tx, "user.created")
})
```

Opening a second transaction on the pool instead is the mistake to avoid: two
transactions in one request do not roll back together.

### Another template engine

`pw.WriteHTML` takes a generated fragment and renders it inside the registered
document shell. A different engine simply skips both — render into a buffer
first so a template error cannot leave a half-written 200:

```go
var body bytes.Buffer
if err := tmpl.ExecuteTemplate(&body, "home.html", data); err != nil {
	pw.WriteProblem(w, r, err)
	return
}
w.Header().Set("Content-Type", "text/html; charset=utf-8")
w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
w.WriteHeader(http.StatusOK)
body.WriteTo(w)
```

`html/template`, and template engines built on it, use reflection — so this is
the point where a TinyGo build stops being possible.

### Writing responses yourself

`json.NewEncoder(w).Encode(value)` and `http.Redirect` work exactly as they do
in any `net/http` application. `pw.WriteProblem` is worth keeping even then: it
accepts any `error`, never leaks 5xx detail, and refuses to append a second
payload to a response that has already been committed.

## Using the helpers without the framework

The generated layers are not part of this repository. They come from
[`tinybind-go`](https://github.com/shibukawa/tinybind-go), which was written for
this framework and is usable on its own:

| Package | What it gives you |
| --- | --- |
| `httpbind` | request binding, validation, JSON responses, streaming, OpenAPI 3.1 |
| `htmlbind` | typed HTML components and render chains |
| `sqlbind` | typed SQL statements and result scanning |
| `configbind` | configuration binding, scaffolds, CLI subcommands |

Generation is per-package and driven by a directive rather than by a project
layout:

```go
package api

//go:generate go run github.com/shibukawa/tinybind-go/cmd/tinybind-gen generate -dir . -openapi
```

Outside a Popcorn Wave project the template suffixes are `.tb.html` and
`.tb.sql` (`-html-template-pattern` and `-sql-template-pattern` change them),
and the output lands in `tinybind_gen.go` and `tinybind_templates_gen.go`.

### Example: the helpers inside Echo

```go
func createUser(c echo.Context) error {
	r, w := c.Request(), c.Response()

	// Echo's router owns path parameters; httpbind reads r.PathValue.
	r.SetPathValue("org_id", c.Param("org_id"))

	input, err := httpbind.Bind[CreateUserRequest](r)
	if err != nil {
		httpbind.WriteError(w, r, err) // RFC 9457, with field detail
		return nil
	}

	ctx := sqlbind.WithSQLExecutor(r.Context(), db)
	user, err := store.InsertUser(ctx, input.Name)
	if err != nil {
		return err
	}

	return httpbind.Write(w, r, user)
}
```

Three details make that work:

- `c.Request()` and `c.Response()` are an ordinary `*http.Request` and
  `http.ResponseWriter`, so every helper takes them unchanged;
- `path:` tags read `r.PathValue`, which only `net/http`'s own mux populates —
  one `SetPathValue` per parameter bridges any other router;
- context-resolved query functions require generating with `-sql-context-api`
  (or `-sql-context-only-api`, which is the form Popcorn Wave itself uses).

HTML works the same way, with the handler keeping control of status and headers:

```go
w.Header().Set("Content-Type", "text/html; charset=utf-8")
err := htmlbind.Render(w, pages.Hello(pages.HelloParams{Name: input.Name}))
```

### The middleware travels too

Everything in
[`middlewares`](https://github.com/shibukawa/popcornwave/tree/main/middlewares)
is a plain `func(http.Handler) http.Handler` with its dependencies passed as
options rather than read from package globals. Any standard-library-compatible
stack takes it directly, and Echo wraps it:

```go
e.Use(echo.WrapMiddleware(middlewares.RequestID()))
e.Use(echo.WrapMiddleware(middlewares.MaxRequestBody(10 << 20)))
```

## What does not travel

| Stays with Popcorn Wave | Why |
| --- | --- |
| the implicit document shell | `pw.WriteHTML` resolves a registered wrapper chain; `htmlbind.RenderChain` needs the chain passed in |
| layered configuration and `--generate-config` | `configbind` binds a struct; the file search order, environment selection, and merged scaffolds are the framework's |
| `/healthz`, `/readyz`, the served OpenAPI document | mounted by `pw.Middlewares` |
| project-wide OpenAPI merge | `tinybind-gen` emits one fragment per package; merging them deterministically is `pw generate` |
| `pw dev`, migrations, seeds, Tailwind, the dev identity provider | tooling, not runtime |

## The TinyGo line

Everything described in the second half of this page keeps a build TinyGo-ready,
because none of it reflects at run time. Everything in the first half — a
reflect-based query builder, an ORM, `html/template`, a hand-rolled
`encoding/json` decode — is a deliberate step off that target for the build that
imports it. Ordinary Go builds do not care either way, which is why these
escape hatches exist at all: the framework's opinion is about what it generates,
not about what you are allowed to import.
