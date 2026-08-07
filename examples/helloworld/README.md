# Hello World

A classic Popcorn Wave example generated with:

```bash
pw init helloworld --tailwind
```

It demonstrates nested HTML templates, Tailwind CSS, an atomic SQLite page-view
counter, and an application-owned configuration binding rendered into the page.

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

## TinyGo

The example is a TinyGo project, so it also builds with that compiler:

```bash
go run ../../cmd/pw prepare
tinygo build -o helloworld ./cmd/helloworld
```

`pw prepare` rather than `pw generate`: this example enables Tailwind, and
`pw generate` writes the generated Go without rebuilding
`public/generated/app.css`. The binary still compiles, so the symptom of using
the narrower command is a page served with a stale stylesheet rather than an
error.

[tinygohelper.go](tinygohelper.go) registers the host networking driver that
TinyGo's `net` package requires. Without it the binary builds and then exits
with `Netdev not set`. Host Go builds skip the file through its `//go:build
tinygo` constraint.

For the full regeneration and Tailwind watch loop:

```bash
devbox shell
pw dev
```

Open <http://localhost:8080>. The local `helloworld.db` file is ignored by Git.
