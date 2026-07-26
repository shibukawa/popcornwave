# Popcorn Wave

<img src="docs/logo.png" alt="Popcorn Wave" width="480">

Popcorn Wave is a small, TinyGo-oriented web application framework built
directly on `net/http`. It handles ordinary document requests, form posts,
redirects, and APIs without shipping a browser runtime.

Documentation: <https://shibukawa.github.io/popcornwave/> (sources in
[`website/`](website/README.md))

## Getting started

```sh
go install github.com/shibukawa/popcornwave/cmd/pw@latest
pw init myapp
cd myapp && pw dev
```

`pw init` scaffolds a runnable project — handler, typed page template, shared
document shell, SQL, migration, error pages, and Devbox — then runs
`go mod tidy` and `pw generate` so it compiles immediately.

## What an application looks like

A page template, in `handlers/home.pw.html`:

```html
package handlers

export component Home(name: string): html {
<h1>Hello, {name}</h1>
}
```

The handler that renders it, in `handlers/home_handler.go`:

```go
package handlers

import (
    "net/http"

    "github.com/shibukawa/popcornwave/pw"
)

type homeInput struct {
    Name string `query:"name" default:"World"`
}

func init() { mux.HandleFunc("GET /", home) }

func home(w http.ResponseWriter, r *http.Request) {
    input, err := pw.Parse[homeInput](r)
    if err != nil {
        pw.WriteProblem(w, r, pw.BadRequest(err))
        return
    }
    pw.WriteHTML(w, r, Home(HomeParams{Name: input.Name}))
}
```

And the entry point:

```go
func main() {
    if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
        log.Fatal(err)
    }
}
```

`Home` and `HomeParams` are generated from the template by `pw generate`, so
changing the template's parameter list stops the handler compiling until it is
updated. The handler never mentions the document shell — `pw.WriteHTML` renders
the page fragment inside the one registered by `templates/document.pw.html`.

## What the framework provides

- standard `http.Handler`, `http.ServeMux`, and middleware composition —
  `pw.NewServeMux` is a type alias of `http.ServeMux` on ordinary Go builds,
  with an equivalent implementation compiled in only for TinyGo;
- typed HTML templates and typed SQL compiled into Go ahead of time, with no
  runtime reflection in the request path;
- typed request binding from the query string, path, headers, and cookies, and
  from JSON, form-urlencoded, and multipart bodies, with declarative validation;
- RFC 9457 problem responses that never leak 5xx detail;
- SSE, NDJSON, and JSON-array streaming with content negotiation;
- OpenAPI 3.1 assembled from the same declarations that drive binding;
- startup validation, health/readiness/OpenAPI endpoints, graceful shutdown,
  and reverse-order resource cleanup;
- request IDs, request-scoped loggers, recovery, request-body limits, and
  validated browser security headers;
- generated mapping through
  [`tinybind-go`](https://github.com/shibukawa/tinybind-go).

Component graphs, patch protocols, hydration, and browser JavaScript are not
dependencies of this package.

## The `pw` command

| Command | Purpose |
| --- | --- |
| `pw init` | create a runnable project |
| `pw generate` | compile `.pw.html` and `.pw.sql` into Go |
| `pw dev` | watch, regenerate, migrate, and restart |
| `pw build` | produce a release binary |
| `pw migrate` | inspect, apply, and roll back migrations |
| `pw seed` | load seed datasets |

## Configuration

`APP_ENV` selects the runtime environment (`dev`, `stg`, `prod`, or any other
lowercase token) and defaults to `dev`. Project-local configuration is read
from `./config.{APP_ENV}.toml` and then `./config/config.{APP_ENV}.toml`; the
user and system configuration directories keep the environment-neutral
`config.toml`. `--config-path` overrides the search entirely.

Any application can print the combined configuration scaffold registered by
every package it imports. Redirect stdout when a file is wanted:

```sh
cd examples/helloworld
go run ./cmd/helloworld --generate-config toml > config.dev.toml
go run ./cmd/helloworld --generate-config env > .env
```

## Testing

Tests can run an application from an isolated copy of every registered
framework and application configuration. The customizer initially sees port
`-1`; `TestRun` reserves an available loopback port before startup.

```go
server := testutil.TestRun(t, handlers.Handlers(), func(config *testutil.Config) {
    testutil.Update[AppConfig](config, func(app *AppConfig) {
        app.Mode = "test"
    })
    testutil.Update[pw.MiddlewareConfig](config, func(middleware *pw.MiddlewareConfig) {
        middleware.RDB.Enabled = true
        middleware.RDB.DSN = "sqlite://:memory:"
        middleware.RDB.MaxOpenConns = 1
        middleware.RDB.MaxIdleConns = 1
    })
}, testutil.WithMigrations("migrations"))
```
