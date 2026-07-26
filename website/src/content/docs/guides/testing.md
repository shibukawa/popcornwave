---
title: Testing
description: Run a real application from an isolated configuration copy with testutil.
sidebar:
  order: 9
---

`github.com/shibukawa/popcornwave/testutil` starts your actual application —
same middleware stack, same configuration binding, same routes — against a copy
of every registered configuration. Tests exercise the real thing over HTTP
rather than a hand-assembled subset of it.

## A first test

```go
func TestHome(t *testing.T) {
	server := testutil.TestRun(t, Handlers(), func(config *testutil.Config) {
		testutil.Update[pw.MiddlewareConfig](config, func(middleware *pw.MiddlewareConfig) {
			middleware.RDB = pw.RDBConfig{
				Enabled:        true,
				DSN:            "sqlite://:memory:",
				ConnectTimeout: time.Second,
				MaxOpenConns:   1,
				MaxIdleConns:   1,
			}
		})
	}, testutil.WithMigrations("../migrations"))

	response, err := server.Client().Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}
```

`TestRun` copies every registered configuration, sets the copied port to `-1`,
applies your customiser, reserves an available loopback port, initialises the
copied runtime resources, and starts the server. Cleanup is registered with
`t`, so there is nothing to defer.

The port detail matters when you write a customiser that reads the port: it sees
`-1` at that moment, because the real port is chosen afterwards. Read
`server.URL` or `server.Port` instead.

## Registering generated packages

Generated registrations — the document shell, config definitions — happen in
package `init`, so the test binary has to link them:

```go
import (
	_ "myapp"            // public.go
	_ "myapp/templates"  // the document shell
)
```

Without these the server starts without a registered document and HTML
rendering fails at startup.

## Customising configuration

| Call | Purpose |
| --- | --- |
| `testutil.Get[T](config)` | read a copied configuration struct |
| `testutil.Set(config, value)` | replace one wholesale |
| `testutil.Update[T](config, fn)` | edit one in place |

Each is generic over the configuration type, so framework and application
settings are reached the same typed way:

```go
testutil.Update[AppConfig](config, func(app *AppConfig) {
	app.EnvLabel = "test"
})
```

## Options

### `WithMigrations` / `WithMigrationsFS`

```go
testutil.WithMigrations("../migrations")
```

Applies the migration set to the copied database before the server starts.
`WithMigrationsFS` takes an `fs.FS` instead, for embedded migrations.

### `WithSeed` / `WithSeedDir`

```go
testutil.WithSeed("initial")
```

Loads datasets after the schema is installed and before the server starts. Names
are relative to the seed directory — `testdata/seed` by default, overridable
with `WithSeedDir` — and the `.yaml` extension may be omitted. Datasets are
applied in the order given.

A dataset file is a table name mapped to rows:

```yaml
member:
- { id: 1, name: Frank }
- { id: 2, name: Grace }
```

These are the same files `pw seed` applies, so a fixture cannot drift between
the CLI and the test suite.

### `WithTransaction`

```go
testutil.WithTransaction(true)
```

Runs every request of the test server inside one transaction that is rolled back
when the test finishes. Tests sharing one database stay independent and can run
in parallel. Transactions the application itself starts nest into it as
savepoints, which requires a driver with savepoint support.

### `WithIdentityProvider`

```go
server := testutil.TestRun(t, handlers.Handlers(), nil, testutil.WithIdentityProvider(
	testutil.WithIdPConfig("../devidp.toml"),
	testutil.WithLoginUser("admin"),
	testutil.WithIdPBinding(func(config *testutil.Config, idp testutil.IdPInfo) {
		testutil.Update[handlers.AuthConfig](config, func(auth *handlers.AuthConfig) {
			auth.Issuer, auth.ClientID, auth.ClientSecret = idp.Issuer, idp.ClientID, idp.ClientSecret
		})
	}),
))
```

Starts the same development identity provider [`pw dev`](/pw/project/dev/) uses,
on its own loopback port, before the application server. `WithLoginUser`
pre-selects the subject, so the authorization endpoint redirects straight back
with a code and the whole login completes without driving a browser:

```go
response, err := server.Client().Get(server.URL + "/login")
```

The roster comes from exactly one of `WithIdPConfig` (a `devidp.toml` file),
`WithIdPRoster` (the same TOML held in the test), or `WithIdPUsers` (Go values).
`WithIdPBinding` writes the issuer and the generated client credentials into the
copied configuration; it runs after `customize`, so it wins over a placeholder.
`server.LoginAs(t, "guest")` switches user mid-test, and `server.IdPInfo()`
returns the same values for a test that wires its client by hand.

## Asserting against the database

`server.Context()` returns a context carrying the same runtime resources the
server installs on requests — including the `WithTransaction` transaction — so
generated queries can prepare or assert data inside the same transaction the
handlers used:

```go
counter, err := queries.CurrentAccess(server.Context())
```

`server.DB` exposes the pool directly when you need raw SQL.
