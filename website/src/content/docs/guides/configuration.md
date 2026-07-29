---
title: Configuration
description: APP_ENV, file resolution order, framework settings, and application-owned config.
sidebar:
  order: 7
---

Configuration comes from several places, but it resolves to one typed view.
Popcorn Wave binds TOML files, environment variables, and command-line flags to
structs before the first request; an invalid value stops startup instead of
surfacing midway through runtime.

## Environments

`APP_ENV` selects the runtime environment. It accepts `dev`, `stg`, `prod`, or
any other token made of lowercase letters, digits, `-`, and `_`. An invalid
token fails `ParseConfig`. When unset or empty it defaults to **`dev`**.

```sh
APP_ENV=prod ./myapp
```

`pw.Env()` returns the resolved token, and `pw.EnvDevelopment`,
`pw.EnvStaging`, and `pw.EnvProduction` name the well-known ones.

## File resolution

Selecting an environment determines the project-local filename. Popcorn Wave
searches the working directory first, then its `config/` directory:

1. `./config.{APP_ENV}.toml`
2. `./config/config.{APP_ENV}.toml`

User and system configuration directories use the environment-neutral
`config.toml`. A project tree does not: a bare `config.toml` there is never
read.

Values later in the chain — environment variables, then flags — override earlier
ones.

## Framework settings

Five prefixes are registered by the framework itself.

### `[server]`

| Key | Default |
| --- | --- |
| `port` | `8080` |
| `read_header_timeout` | `5s` |
| `read_timeout` | `30s` |
| `write_timeout` | `0s` (unbounded, permits long-lived streams) |
| `idle_timeout` | `2m` |
| `shutdown_timeout` | `10s` |
| `max_request_body` | `10485760` |
| `trusted_proxies` | *(empty)* |
| `health.enabled` / `health.path` | `true` / `/healthz` |
| `readiness.enabled` / `readiness.path` | `true` / `/readyz` |
| `openapi.enabled` / `openapi.path` | `true` / `/openapi.json` |
| `api_doc` / `api_doc_path` | *(empty)* / `/docs` |
| `public.enabled` / `public.mount` | `true` / `/public` |
| `public.read_local` | `false` |

`openapi.*` serves the generated OpenAPI document and `api_doc` adds a browsable
UI over it, which requires `openapi.enabled`. See
[API Documentation](/productivity/api-documentation/).

If an application route collides with an enabled operational endpoint, startup
fails before either route can shadow the other.

### `[middleware]`

| Key | Default |
| --- | --- |
| `recovery` | `true` |
| `request_id` | `true` |
| `access_log` | `true` |
| `compression` | `false` |
| `request_timeout` | `0s` |
| `rdb.enabled` | `false` |
| `rdb.dsn` | *(empty)* |
| `rdb.connect_timeout` | `5s` |
| `rdb.max_open_conns` / `rdb.max_idle_conns` | `0` |
| `rdb.conn_max_lifetime` / `rdb.conn_max_idle_time` | `0s` |

With `compression` enabled, HTML responses are zstd-encoded for clients that
accept it, and `Vary: Accept-Encoding` is set either way.

### `[security]`

`headers.enabled` (`true`), `headers.content_type_options` (`true`),
`headers.frame_options` (`deny`), `headers.referrer_policy`
(`strict-origin-when-cross-origin`), `headers.content_security_policy`,
`headers.content_security_policy_report_only`, `headers.permissions_policy`, and
an `headers.hsts` block (disabled by default) applied only on verified HTTPS
requests.

### `[observability]`

`minimum_level` (`info`), `service_name`, which also reads `OTEL_SERVICE_NAME`,
and `boot_log` (`auto`), which selects the [startup summary](/productivity/startup-summary/)
format.

`[observability.query]` logs each generated SQL statement, explains the slow
ones, and prints a rerun snippet: `enabled` and `bind_values` (`auto` — on in
`dev`, off elsewhere), `level` (`info`), `slow_threshold` (`200ms`),
`slow_level` (`warn`), `explain` and `reproduction` (`true`), and the
`max_sql_length` (`4096`) and `max_value_length` (`256`) bounds. See
[Query Diagnostics](/productivity/query-diagnostics/).

### `[session]`

`enabled` (`false`), `ttl` (`24h`), and `secret`, which also reads
`SESSION_SECRET`.

### `[auth]`

`enabled` (`false`) and `mode` — `oidc`, `oidc_passkey`, or `passkey_only` —
plus `login_path`, `callback_path`, `logout_path`, `post_login_redirect`, and
`post_logout_redirect`; and the `[auth.oidc]` provider settings: `issuer`,
`client_id`, `client_secret`, `redirect_url`, `scopes`, and `provider_logout`
(default `true`, which also ends the provider session on logout).

Each provider value also reads an environment variable: `AUTH_OIDC_ISSUER`, `AUTH_OIDC_CLIENT_ID`,
`AUTH_OIDC_CLIENT_SECRET`, and `AUTH_OIDC_REDIRECT_URL`.

An OIDC mode with an empty `issuer`, `client_id`, or `client_secret` fails at
startup rather than at the first login. The error names the missing keys and
their environment variables:

```
auth.mode "oidc" needs auth.oidc.issuer (AUTH_OIDC_ISSUER), auth.oidc.client_id
(AUTH_OIDC_CLIENT_ID); run pw dev to use the development identity provider, or
supply the values in config.dev.toml or the environment
```

That is why a project scaffolded for the local emulator carries no provider
values at all — [`pw dev`](/pw/project/dev/) injects them, and a run without it
tells you exactly what is missing.

## Adding your own settings

Application settings follow the same path as framework settings: declare a
struct, register it under a prefix, and read the result from the request
context. `pw generate` turns the registration call into binding code, so adding
a setting does not add a parallel parser.

### 1. Declare the struct

```go
package handlers

import "github.com/shibukawa/popcornwave/pw"

type AppConfig struct {
	EnvLabel      string `default:"local" help:"environment name shown in the page badge"`
	EnvLabelColor string `default:"#64748b" help:"CSS color of the environment badge"`
}
```

Field names become snake_case keys, so `EnvLabel` is `app.env_label`. Five tags
adjust that:

| Tag | Effect |
| --- | --- |
| `default:"value"` | value when nothing else supplies one |
| `key:"name"` | override the stable TOML/config key |
| `opt:"long"` / `opt:"long,s"` | override the CLI option, optionally with a short form |
| `env:"NAME"` / `env:"-"` | exact environment variable name, or disable environment input |
| `help:"text"` | description shown in usage and scaffolds |

Nested structs nest the key. With prefix `app` and

```go
type AppConfig struct {
	Mailer MailerConfig
}

type MailerConfig struct {
	FromAddress string `default:"noreply@example.com"`
}
```

the key is `app.mailer.from_address`, the TOML is `[app.mailer]
from_address = …`, the option is `--app-mailer-from_address`, and the
environment variable is `APP_MAILER_FROM_ADDRESS`.

:::caution
Supported field types are `string`, `bool`, `int`, `[]string`, and nested
structs of those. Floats, maps, pointers, other slice types, and
`time.Duration` are **not** bindable — declare them as `string` or `int` and
convert after parsing. (The framework's own `[server]` durations work because
those bindings are hand-written, not generated.)
:::

### 2. Register it

```go
func RegisterConfig() { pw.RegisterConfig[AppConfig]("app") }
```

```go
func main() {
	handlers.RegisterConfig()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

Timing defines whether the generator can assemble the complete configuration.
Generated definitions register during package `init`, so the binding must be
created **after** every `init` has run but **before** parsing begins.
Registration after `ParseConfig` panics, and the prefix must be a string literal
that the generator can read.

Each area of a larger application can register its own struct — see
[Project structure](/guides/project-structure/) — but prefixes share one
namespace, so give them distinct names (`app`, `billing`, `search`).

### 3. Read it

```go
app := pw.Config[AppConfig](r.Context())
```

`pw.Config` is available anywhere a request context is, and takes `nil` outside
a request.

### 4. Set it

```toml
[app]
env_label = "development"
env_label_color = "#059669"
```

```sh
APP_ENV_LABEL=development ./myapp
./myapp --app-env_label=development
```

## Generating a scaffold

Every registered prefix — framework and application alike — can print itself,
with `default` values filled in and `help` text as comments:

```sh
./myapp --generate-config toml > config.dev.toml
./myapp --generate-config env > .env
```

Because the binary reports registrations from its actual imports, the scaffold
matches the packages linked into that build. Add a struct and rerun the command;
the new keys appear. See [Application CLI](/guides/application-cli/).

## Startup summary

Resolved configuration is reported once at startup — as a tree on a terminal, as
one structured record everywhere else — so you can see which values actually
took effect and where each came from. `observability.boot_log` selects the
format. See [Startup Summary](/productivity/startup-summary/).
