# Petitweb for Go

<img src="docs/logo.png" alt="Petitweb" width="480">

Petitweb is a small, TinyGo-oriented web application framework built directly
on `net/http`. Classic mode handles ordinary document requests, form posts,
redirects, downloads, and APIs without shipping a browser runtime.

```go
app := petitweb.New(
    petitweb.WithMiddleware(
        petitweb.RequestID("", nil),
        petitweb.Recover(petitweb.ErrorHandler{}),
    ),
)

app.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
    err := petitweb.HTML(w, http.StatusOK, petitweb.RenderFunc(func(w io.Writer) error {
        _, err := io.WriteString(w, "<!doctype html><h1>Hello</h1>")
        return err
    }))
    if err != nil {
        app.WriteError(w, r, err)
    }
})

log.Fatal(app.ListenAndServe(":8080"))
```

The runtime provides:

- standard `http.Handler`, `http.ServeMux`, and middleware composition;
- startup validation, health/readiness/OpenAPI endpoints, graceful shutdown,
  and reverse-order resource cleanup;
- safe RFC 9457 or application-supplied HTML error rendering;
- complete HTML, JSON, XML, CSV, redirect, and download responses;
- request IDs, request-scoped loggers, recovery, request-body limits, and
  validated browser security headers;
- generated typed request/response mapping through
  [`tinybind-go`](https://github.com/shibukawa/tinybind-go).

Modern component graphs, patch protocols, hydration, and browser JavaScript are
not dependencies of this package.

## Authentication

Opaque server-side login sessions live in [`session`](session/README.md), and
[`plugin/session/rdb`](plugin/session/rdb/README.md) stores them in a
`database/sql` database. Importing
[`plugin/auth`](plugin/auth/README.md) adds the `[auth]` binding and the login
flow itself; nothing is installed until `auth.enabled` is true, so an unused
import costs one configuration binding.

The implemented mode is `auth.mode = "oidc_only"`: OpenID Connect
Authorization Code with PKCE over [`contrib/oidc`](contrib/oidc/README.md).
[`examples/oidclogin`](examples/oidclogin/README.md) is a complete application
that logs in against a local [oidcld](https://github.com/shibukawa/oidcld)
provider and keeps its sessions in SQLite.

The Hello World example can print combined configuration scaffolds registered
by every imported package. Redirect stdout when a file is wanted:

```sh
cd examples/helloworld
go run ./cmd/helloworld --generate-config toml > config.dev.toml
go run ./cmd/helloworld --generate-config env > .env
```

`APP_ENV` selects the runtime environment (`dev`, `stg`, `prod`, or any other
lowercase token) and defaults to `dev`. Project-local configuration is read
from `./config.{APP_ENV}.toml` and then `./config/config.{APP_ENV}.toml`; the
user and system configuration directories keep the environment-neutral
`config.toml`. `--config-path` overrides the search entirely.

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
