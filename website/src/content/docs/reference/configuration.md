---
title: Configuration Keys
description: Every framework configuration key, its default, and the TOML, environment, and command-line names it answers to.
sidebar:
  order: 2
---

One struct field becomes three inputs. `ServerConfig.ReadHeaderTimeout` is
`server.read_header_timeout` in TOML, `SERVER_READ_HEADER_TIMEOUT` in the
environment, and `--server-read_header_timeout` on the command line — and the
last two are derived from the first by rule, not by a table someone maintains.
So the listings below name each key once, and you can compute the other two
forms yourself.

Rules have exceptions, and this one has a handful worth knowing before the
tables. For how configuration is *used* — declaring your own settings, reading
them in a handler, generating a scaffold — see
[Configuration](/guides/architecture/configuration/).

## Deriving the other two names

Take the TOML key, replace each `.` with `-`, and prefix `--`:
`observability.query.slow_threshold` becomes
`--observability-query-slow_threshold`. Underscores inside a key survive; only
the dots that separate nesting levels change.

Take that option name, drop the dashes, and upcase:
`OBSERVABILITY_QUERY_SLOW_THRESHOLD`.

Five keys break the rule on purpose, because a conventional name already exists
and an application should not have to translate:

| Key | Environment variable |
| --- | --- |
| `server.port` | `PORT` (option is `--port`) |
| `observability.service_name` | `OTEL_SERVICE_NAME` |
| `observability.otel.endpoint` | `OTEL_EXPORTER_OTLP_ENDPOINT` |
| `observability.otel.headers` | `OTEL_EXPORTER_OTLP_HEADERS` |
| `auth.oidc.issuer`, `client_id`, `client_secret`, `redirect_url` | `AUTH_OIDC_*` — the rule's own result, fixed rather than derived |

Three keys have no environment binding at all:
`security.headers.content_security_policy`, its `_report_only` twin, and
`security.headers.permissions_policy`. Set them in TOML.

The `[[middleware.rdb.connections]]` array is TOML-only as well. An array of
tables has no flat name to bind.

## Where values come from

`APP_ENV` selects the environment and therefore the project-local filename.
Popcorn Wave reads, in order:

1. `./config.{APP_ENV}.toml`
2. `./config/config.{APP_ENV}.toml`
3. the user and system configuration directories, where the file is the
   environment-neutral `config.toml`

A bare `config.toml` in the project tree is never read. Later sources override
earlier ones, and environment variables and flags override every file.

Durations are Go duration strings: `5s`, `200ms`, `2m`, `24h`. Sizes are plain
integers of bytes. A list is a TOML array, or a comma-separated value in an
environment variable.

## `[server]`

| Key | Default | Meaning |
| --- | --- | --- |
| `port` | `8080` | HTTP listen port |
| `read_header_timeout` | `"5s"` | request header read timeout |
| `read_timeout` | `"30s"` | request read timeout |
| `write_timeout` | `"0s"` | response write timeout; zero permits long-lived streams |
| `idle_timeout` | `"2m"` | keep-alive idle timeout |
| `shutdown_timeout` | `"10s"` | graceful shutdown timeout |
| `max_request_body` | `10485760` | maximum request body in bytes |
| `trusted_proxies` | `[]` | trusted proxy IP or CIDR |
| `health` | *(empty)* | liveness endpoint path, e.g. `/healthz` |
| `readiness` | *(empty)* | readiness endpoint path, e.g. `/readyz` |
| `openapi` | *(empty)* | OpenAPI document path, e.g. `/openapi.json` |
| `api_doc` | *(empty)* | API documentation UI: `scalar`, `swagger`, or empty |
| `api_doc_path` | `"/docs"` | where that UI is mounted |
| `public.enabled` | `true` | serve the embedded static assets |
| `public.mount` | `"/public"` | where they are mounted |
| `public.read_local` | `false` | read from disk instead of the embedded tree |

The three operational endpoints carry no default path, and that is deliberate:
an application answering on `/healthz` should say so where an operator reading
its configuration will see it. A default would leave three endpoints running
that no file mentions.

An application route colliding with an enabled operational endpoint fails
startup, before either can shadow the other. `api_doc` additionally requires
`openapi` — a UI over a document nobody serves has nothing to render. See
[API Documentation](/productivity/api-documentation/).

## `[middleware]`

| Key | Default | Meaning |
| --- | --- | --- |
| `recovery` | `true` | recover a panicking handler into a 500 |
| `request_id` | `true` | assign and propagate a request correlation ID |
| `access_log` | `true` | one record per request |
| `compression` | `false` | zstd-encode HTML for clients that accept it |
| `request_timeout` | `"0s"` | per-request deadline; zero leaves none |
| `rdb.enabled` | `false` | open the framework-owned database pool |
| `rdb.dsn` | *(empty)* | data source name for a single database (masked in the startup summary) |
| `rdb.connect_timeout` | `"5s"` | bound on opening a connection |
| `rdb.max_open_conns` | `0` | `database/sql` pool bounds; zero means the driver default |
| `rdb.max_idle_conns` | `0` | |
| `rdb.conn_max_lifetime` | `"0s"` | |
| `rdb.conn_max_idle_time` | `"0s"` | |
| `rdb.default_group` | *(empty)* | connection group for statements that pin none |
| `rdb.write_group` | *(empty)* | connection group for framework-owned writes |
| `rdb.migration_group` | *(empty)* | connection group for migrations and seeds |

With `compression` enabled, `Vary: Accept-Encoding` is set either way — a cache
that saw one representation must not serve it to a client that asked for the
other.

A single database is configured with `rdb.dsn` and the pool keys above. A
reader-writer topology is configured with the connection set instead, one table
per pool. Declaring both is a configuration error rather than a merge, because
there is no honest answer to which one wins.

### `[[middleware.rdb.connections]]`

| Key | Default | Meaning |
| --- | --- | --- |
| `group` | *(empty)* | the name this connection is addressed by |
| `dsn` | *(empty)* | data source name (masked in the startup summary) |
| `readonly` | `false` | open read-only transactions and serve no framework write |
| `connect_timeout` | `"5s"` | as above, per connection |
| `max_open_conns` | `0` | |
| `max_idle_conns` | `0` | |
| `conn_max_lifetime` | `"0s"` | |
| `conn_max_idle_time` | `"0s"` | |

A `readonly` connection can never be selected by `pw.SelectWriteDB`, which is
what lets a caller that must write stay ignorant of the topology. See
[Queries](/guides/backend/queries/).

## `[html]`

| Key | Default | Meaning |
| --- | --- | --- |
| `streaming` | `true` | `false` forces the buffered branch even when a chain could stream |
| `async_timeout` | `"3s"` | bound on one await boundary; zero leaves the request context as the only deadline |
| `async_concurrency` | `0` | simultaneous boundary work per render; zero or less is unbounded |
| `bot_detection` | `true` | render the settled document for crawlers and CLI clients |
| `bot_async_timeout` | `"5s"` | boundary bound on a classified bot request |
| `bot_user_agents` | `[]` | additional `User-Agent` substrings, matched case-insensitively |

A template that opens an await boundary renders correctly under either
`streaming` setting. The key decides only whether the fallbacks reach the
browser before the work behind them settles, which makes `streaming = false` the
escape hatch for a proxy that buffers responses anyway.

`bot_async_timeout` sits above `async_timeout` because a bot request waits for
every boundary before any byte leaves, and an indexer waits longer than a
browser. A zero here falls back to `async_timeout` rather than meaning
unbounded: a mistyped key must not hold a crawler's connection open for the
whole request deadline. Entries in `bot_user_agents` are appended to the
built-in catalog and never replace a built-in token.

## `[security]`

| Key | Default | Meaning |
| --- | --- | --- |
| `headers.enabled` | `true` | the switch every key below answers to |
| `headers.content_type_options` | `true` | `X-Content-Type-Options: nosniff` |
| `headers.frame_options` | `"deny"` | `X-Frame-Options` |
| `headers.referrer_policy` | `"strict-origin-when-cross-origin"` | `Referrer-Policy` |
| `headers.content_security_policy` | *(empty)* | `Content-Security-Policy` (no environment binding) |
| `headers.content_security_policy_report_only` | *(empty)* | the report-only variant (no environment binding) |
| `headers.permissions_policy` | *(empty)* | `Permissions-Policy` (no environment binding) |
| `headers.hsts.enabled` | `false` | `Strict-Transport-Security`, on verified HTTPS requests only |
| `headers.hsts.max_age` | `"0s"` | |
| `headers.hsts.include_subdomains` | `false` | |
| `headers.hsts.preload` | `false` | |

HSTS is applied only on a verified HTTPS request. Sending it over plaintext
would ask a browser to remember a policy the connection could not vouch for.

## `[observability]`

| Key | Default | Meaning |
| --- | --- | --- |
| `minimum_level` | `"info"` | severity floor: `trace`, `debug`, `info`, `warn`, `error`, `off` |
| `stdout_format` | `"json"` | terminal record encoding: `json` or `plaintext` |
| `service_name` | *(empty)* | also read from `OTEL_SERVICE_NAME` |
| `resource_attributes` | `[]` | extra `key=value` identifiers reported with the service name |
| `boot_log` | `"auto"` | startup summary: `auto`, `tree`, `record`, `off` |

`auto` renders the tree on an interactive terminal and one structured record
everywhere else. See [Startup Summary](/productivity/startup-summary/).

### `[observability.query]`

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `"auto"` | log every generated statement: `auto`, `on`, `off` — `auto` is on in `dev` |
| `level` | `"info"` | severity of an ordinary statement record |
| `slow_threshold` | `"200ms"` | duration above which a statement is slow; zero disables slow detection |
| `slow_level` | `"warn"` | severity of a slow statement record |
| `bind_values` | `"auto"` | log argument values: `auto`, `on`, `off` |
| `explain` | `true` | capture a plan-only `EXPLAIN` for a slow statement |
| `reproduction` | `true` | emit a paste-able rerun snippet for a slow statement |
| `max_sql_length` | `4096` | bound on the logged statement text |
| `max_value_length` | `256` | bound on each logged argument value |

`auto` ties the setting to the environment, so a development run is instrumented
without configuration and every other environment stays silent until someone
opts in. `explain` and `reproduction` depend on `slow_threshold`, not on
`enabled`: setting the threshold to zero switches off all three at once. See
[Query Diagnostics](/productivity/query-diagnostics/).

### `[observability.otel]`

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `false` | export traces and logs |
| `endpoint` | *(empty)* | OTLP/HTTP base URL; `/v1/traces` and `/v1/logs` are appended |
| `headers` | *(empty)* | comma-separated `key=value` list; values are never logged |
| `request_timeout` | `"10s"` | bound on one export request |
| `queue_size` | `2048` | records held in memory; a full queue drops rather than blocking the request |
| `max_export_size` | `512` | bound on one exported batch |
| `flush_interval` | `"5s"` | how often a partial batch is sent |

These defaults restate the bounds the exporter applies to a zero value, so a
scaffolded file says what the process will do instead of showing a zero that
means "ask someone else".

`OTEL_EXPORTER_OTLP_TIMEOUT` is deliberately **not** bound to `request_timeout`.
The standard variable counts milliseconds, while every duration here is a Go
duration string, and one key cannot mean both.

## `[session]`

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `false` | |
| `backend` | `"rdb"` | storage backend: `rdb`, `cookie`, or `redis` |
| `ttl` | `"24h"` | absolute session lifetime |
| `idle_timeout` | `"0s"` | inactivity expiry; zero disables it |
| `renewal_interval` | `"0s"` | minimum interval between idle expiry renewals |
| `cookie.name` | `"pw_session"` | |
| `cookie.path` | `"/"` | |
| `cookie.domain` | *(empty)* | |
| `cookie.secure` | `true` | disable only for loopback development |
| `cookie.http_only` | `true` | |
| `cookie.same_site` | `"lax"` | |
| `rdb.source` | `"middleware"` | `middleware` reuses the `middleware.rdb` pool; `dedicated` opens `rdb.dsn` |
| `rdb.group` | *(empty)* | connection group holding the session table; empty resolves to `middleware.rdb.write_group` |
| `rdb.dsn` | *(empty)* | dedicated session database (masked in the startup summary) |
| `rdb.table` | `"popcornwave_session"` | |
| `redis.dsn` | *(empty)* | `redis://` or `rediss://` server (masked in the startup summary) |
| `redis.key_prefix` | `"pw:session:"` | key space the session store owns |
| `redis.connect_timeout` | `"5s"` | startup ping and per-command deadline |
| `cookie_store.name` | `"pw_session_data"` | cookie holding the sealed record under `backend = "cookie"` |
| `cookie_store.secret` | *(empty)* | base64 secret sealing cookie-backed records (masked) |
| `cookie_store.previous_secrets` | `[]` | retired secrets kept readable during a rotation (masked) |

Only the keys of the selected backend are read, and a backend other than
`cookie` reaches the binary through its own blank import — the startup error
quotes the line to add. [Sessions](/guides/backend/sessions/) compares the
three and lists what each one requires.

The token in the browser is opaque in all three, so nothing here signs it. Only
`backend = "cookie"` puts the record itself in the browser, and it seals that
record under `cookie_store.secret` — the one secret this section has, and one
that belongs in the environment rather than in the file.

## `[auth]`

These keys exist only when `plugin/auth` is linked into the binary, which
happens when the application registers an account resolver. An application that
imports nothing authentication-related has no `[auth]` prefix to configure.

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `false` | |
| `mode` | `"oidc_only"` | |
| `login_path` | `"/auth/login"` | starts the provider flow |
| `callback_path` | `"/auth/callback"` | verifies the result and starts the session |
| `logout_path` | `"/auth/logout"` | ends the session; `POST` only |
| `post_login_path` | `"/"` | local path a completed login lands on |
| `protection.include` | `[]` | path patterns that require a session |
| `protection.exclude` | `[]` | path patterns that stay public |
| `protection.unauthenticated` | `"redirect"` | `redirect` or `unauthorized` |

### `[auth.oidc]`

| Key | Default | Meaning |
| --- | --- | --- |
| `issuer` | *(empty)* | `AUTH_OIDC_ISSUER` |
| `client_id` | *(empty)* | `AUTH_OIDC_CLIENT_ID` |
| `client_secret` | *(empty)* | `AUTH_OIDC_CLIENT_SECRET` (masked in the startup summary) |
| `redirect_url` | *(empty)* | `AUTH_OIDC_REDIRECT_URL` |
| `scopes` | `[]` | |
| `identity_claim` | `"sub"` | the verified claim that identifies a local account |
| `admission` | `"authenticated"` | `authenticated`, `claim`, `registered`, or `existing` |
| `auto_provision` | `true` | let an unknown verified identity create an account through the resolver |
| `claim.path` | *(empty)* | JSON Pointer into verified claims, for `admission = "claim"` |
| `claim.values` | `[]` | accepted values |
| `claim.match` | `"any"` | `any` or `all` |
| `registered_claims` | `[]` | claims compared against the allowlist; defaults to `identity_claim` |
| `provider_logout` | `true` | also end the provider session on logout |
| `allow_loopback_http` | `false` | permit an `http` loopback issuer during development |

`identity_claim` becomes the account link, so whatever it names must be stable
for the life of the account and unique within the issuer. A reissued or reused
value hands one person another person's account. A deployment that provisions
users in advance usually cannot know a subject yet and points this at its own
directory identifier, such as an employee number.

An enabled OIDC mode with an empty `issuer`, `client_id`, or `client_secret`
fails at startup rather than at the first login, and the error names both the
missing keys and their environment variables. That is why a project scaffolded
for the local emulator carries no provider values at all —
[`pw dev`](/pw/project/dev/) injects them. See
[Authentication](/guides/backend/authentication/).

## A key you set that the startup summary does not show

Many keys above answer to a parent switch. `server.api_doc_path` depends on
`server.api_doc`; every `[auth.oidc]` key depends on `auth.enabled`;
`observability.otel.endpoint` depends on `otel.enabled`.

When the parent is empty or false, the summary omits the dependents and keeps
the parent — an empty parent is the reason its dependents vanished, and printing
seven keys that decide nothing would bury the one that does. Binding is
unaffected: the value was read, it simply has nothing to act on yet. So a key
you set and cannot find in the summary is a question about its parent, not about
your spelling.

## The list your binary actually has

The tables here cover the framework and the authentication plugin. Your build
also carries whatever your own packages registered, and only the binary knows
the union:

```sh
./myapp --generate-config toml > config.dev.toml
./myapp --generate-config env > .env
```

Because the scaffold is assembled from registrations present in that build, it
is the authoritative list for that binary — including your `[app]` prefix and
excluding any framework capability you never imported. Adding a struct and
rerunning the command is how the new keys appear; nothing here needs editing.
See [Custom Commands](/guides/architecture/custom-commands/).
