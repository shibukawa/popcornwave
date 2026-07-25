# Petitweb for Go

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

The Hello World example can print combined configuration scaffolds registered
by every imported package. Redirect stdout when a file is wanted:

```sh
cd examples/helloworld
go run ./cmd/helloworld --generate-config toml > config.toml
go run ./cmd/helloworld --generate-config env > .env
```

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
