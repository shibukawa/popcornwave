---
title: Interoperability
description: Which parts are the framework, which are replaceable helpers, and how to use those helpers from another framework.
sidebar:
  order: 1
---

Popcorn Wave generates request parsing, database queries, HTML, and response
writing, so it can look as though the runtime depends on all four. It does not.
The framework's required surface is smaller: a middleware stack over `net/http`
and tooling that keeps generation, development, and builds consistent. The
generated layers are **helpers** that you may replace — or use from another
framework without bringing Popcorn Wave with them.

## Why these layers exist at all

Most Go libraries reach for `reflect` for good reasons. Reflection lets a
request bind into an arbitrary struct, rows map onto a model, and a template walk
unknown data. But that flexibility meets a hard constraint in **TinyGo**:
support is partial, and successful builds still pay in binary size. A framework
that treats TinyGo as a real target needs the same capabilities by another
route.

Each helper is therefore a **code generator**. The work that reflection would
repeat on every request happens once, ahead of time, through
[`pw generate`](/pw/project/generate/): binders for the types you actually
bind, encoders for the types you actually write, scanners for the columns your
SQL actually selects. Nothing in the hot path inspects a type at run time.

The more consequential result is not performance but ownership. A generator
knows only what it can read in source, yet the framework does not compensate by
taking over the handler signature. A handler remains an `http.HandlerFunc`, and
`w` and `r` remain standard types. When generation does not fit an endpoint, you
can write it by hand — or use a reflection-based library and knowingly give up
the TinyGo target for that build.

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

The helpers reduce four common jobs to a small amount of code and feed the
generated OpenAPI document. Those benefits do not turn them into runtime
contracts.

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

The returned value is the escape hatch. Put it behind your own `http.Server`,
another listener, `httptest`, a Lambda adapter, or a host process that serves
other workloads too.

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

The handler still works. What disappears is the work generation performed:
`check` rules, field-level problem details, and the endpoint's request schema in
OpenAPI. That trade can be reasonable for one unusual endpoint and expensive
across forty.

### Queries with sqlx, GORM, or plain `database/sql`

On SQLite and MySQL the pool is a `*sql.DB` and it is available from the
request context:

```go
db, ok := pw.DB(r.Context())
if !ok {
	pw.WriteProblem(w, r, pw.ServiceUnavailable("database unavailable"))
	return
}
users := sqlx.NewDb(db, driver) // or gorm.Open(postgres.New(postgres.Config{Conn: db}))
```

On PostgreSQL `ok` is always `false`: the framework serves requests through a
native pgx pool, and there is no `*sql.DB` behind it to lend out. A library
that requires one gets its own pool instead — open it once at startup with
`stdlib.Open` from `github.com/shibukawa/tinygodriver/database/pgx/stdlib`,
using the same DSN, and own its lifecycle yourself. Keep writes that must
share a transaction with generated statements inside the generated layer; the
two pools cannot join one transaction.

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

Do not replace this with a second transaction on the pool. Two transactions in
one request have independent commit and rollback boundaries.

The `*sql.Tx` assertion holds on SQLite and MySQL. On PostgreSQL the executor
in the context is the native pgx transaction, so code that needs the concrete
handle asserts against the executor interfaces instead — or better, stays in
the generated layer, which never needs the assertion at all.

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
| the operational endpoints (`server.health`, `server.readiness`, `server.openapi`) | mounted by `pw.Middlewares` at the paths a deployment configures |
| project-wide OpenAPI merge | `tinybind-gen` emits one fragment per package; merging them deterministically is `pw generate` |
| `pw dev`, migrations, seeds, Tailwind, the dev identity provider | tooling, not runtime |

## The TinyGo line

The boundary is now concrete. Using the standalone generated helpers keeps a
build TinyGo-ready because they do not reflect at runtime. A reflection-based
query builder, an ORM, `html/template`, or a handwritten `encoding/json` decode
steps off that target for the build that imports it. Ordinary Go builds permit
either choice. Popcorn Wave constrains what it generates, not what an
application is allowed to import.
