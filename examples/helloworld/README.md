# Hello World

A classic Popcorn Wave example generated with:

```bash
pw init helloworld --tailwind
```

It demonstrates nested HTML templates, Tailwind CSS, an atomic SQLite page-view
counter, an application-owned configuration binding rendered into the page, and
an OIDC login served entirely by the framework.

The module always uses the repository checkout through:

```go
replace github.com/shibukawa/popcornwave => ../../
```

Initialize the configured SQLite database, then run the application:

```bash
go run ../../cmd/pw schema-init
go run ./cmd/helloworld
```

## Environment switching

`APP_ENV` selects `config.dev.toml`, `config.stg.toml`, or `config.prod.toml`
from this directory and defaults to `dev`. Staging listens on `8081`, and
staging and production enable compression and HSTS:

```bash
APP_ENV=stg go run ./cmd/helloworld
```

Each file also sets the application-owned `[app]` table, and the home page
renders it as a colored badge:

| `APP_ENV` | `app.env_label` | `app.env_label_color` |
| --- | --- | --- |
| `dev` | `development` | `#059669` |
| `stg` | `staging` | `#d97706` |
| `prod` | `production` | `#dc2626` |

`AppConfig` in [handlers/appconfig.go](handlers/appconfig.go) is a plain struct;
`pw generate` discovers the `pw.RegisterConfig[AppConfig]("app")` call and emits
its TOML, environment, and CLI bindings. The registration is called from `main`
rather than an `init` function, because the generated definition itself
registers during package initialization. Both fields can be overridden without
editing a file:

```bash
APP_ENV=stg APP_ENV_LABEL=canary APP_ENV_LABEL_COLOR="#7c3aed" go run ./cmd/helloworld
go run ./cmd/helloworld --app-env_label=hotfix --app-env_label_color="#0891b2"
```

For the full regeneration and Tailwind watch loop:

```bash
devbox shell
pw dev
```

Open <http://localhost:8080>. The local `helloworld.db` file is ignored by Git.

## Sign in

`pw dev` also starts the development identity provider declared by `[dev.idp]`
and injects `AUTH_OIDC_ISSUER`, `AUTH_OIDC_CLIENT_ID`, and
`AUTH_OIDC_CLIENT_SECRET` into the application, so no credential lives in this
repository. Click **Sign in**, pick `Administrator` or `Member` from
[devidp.toml](devidp.toml), and the page greets you by name. **Sign out** posts
to `/auth/logout`, which accepts POST only.

The application contains no authentication code. [`cmd/helloworld/main.go`](cmd/helloworld/main.go)
imports the package that registers the endpoints:

```go
import _ "github.com/shibukawa/popcornwave/auth"
```

`[auth]` in [config.dev.toml](config.dev.toml) supplies the rest, and the home
handler reads the result with `pw.CurrentUser(r.Context())`.
[handlers/login_test.go](handlers/login_test.go) drives the whole flow without a
browser.

`config.stg.toml` and `config.prod.toml` keep `auth.enabled = false`, because
this sample has no deployed provider. Enabling it there requires
`AUTH_OIDC_ISSUER`, `AUTH_OIDC_CLIENT_ID`, `AUTH_OIDC_CLIENT_SECRET`, and
`SESSION_SECRET`; the application refuses to start while any of them is empty.
