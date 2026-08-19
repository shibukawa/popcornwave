# Configuration

Popcorn Wave has two configuration files read by two different programs at two different times: `popcornwave.toml` (build config, read by the `pw` command) and `config.{APP_ENV}.toml` (runtime config, read by the application binary). The split is enforced — a `[server]` or `[session]` table in `popcornwave.toml` is an error, and so is a DSN there. This file covers both, plus how application code declares its own typed configuration.

## popcornwave.toml (build config)

Sits at the project root, belongs to `pw`, and locates the project (commands walk up until they find it). Unknown keys are **errors**, not warnings. Relative paths resolve from the file's directory; absolute paths are rejected. Command flags override the file for one run.

### `[project]`

| Key | Default | Meaning |
| --- | --- | --- |
| `name` | *(required)* | project name; also the `OTEL_SERVICE_NAME` `pw dev` injects |
| `kind` | `"application"` | `application` builds a binary; `package` is a published Go module |
| `main` | *(required for an application)* | the package `pw build` compiles, e.g. `"./cmd/myapp"` |
| `toolchain` | `"tinygo"` | the compiler the sources were scaffolded for: `tinygo` or `go` |
| `database` | `"sqlite"` | the SQL dialect `.pw.sql` generates for: `sqlite`, `postgres`, or `mysql` |

`database` is a *generation* input: the engine actually connected to comes from the `[[middleware.rdb.connections]]` DSN scheme in the runtime file. Keeping the two in agreement is on you. `kind = "package"` carries no `main` and adds a `[package]` section; an application with a `[package]` section is an error.

### `[generate]`

```toml
[generate]
handlers = ["handlers"]
templates = ["handlers", "templates"]
queries = ["queries"]
config = ["cmd/myapp"]
pages = []
dynamo = []
firestore = []
```

Each purpose lists the directories `pw generate` may read for that purpose and nothing else. `handlers`, `templates`, `queries`, `config` are required with no default; `[]` means "deliberately generates nothing". `pages`, `dynamo`, `firestore` are optional (added by `pw add dynamo` / `pw add firestore` or when using discovered routing). Entries are project-relative existing directories, walked recursively, with no duplicates and no nesting between entries of the same purpose. Exactly one `templates` entry holds the document shell. See references/architecture.md for the full model.

### `[dev.watch]`

```toml
[dev.watch]
includes = []
excludes = []
```

Both optional — `pw dev` already walks the module for rebuild inputs. `includes` adds relative files or glob patterns the walk misses (`["config.dev.toml", "assets/**/*.svg"]`); `excludes` skips a subtree that only slows the walk. Absolute paths are rejected.

### `[dev.idp]` and `[dev.otel]`

Development-only companions `pw dev` runs beside the application; they affect nothing else.

```toml
[dev.idp]
enabled = false
config = "devidp.toml"   # roster file; must exist when enabled
port = 0                 # 0 reserves a free loopback port; pw dev injects the issuer

[dev.otel]
enabled = true           # the telemetry viewer, on by default
port = 0
max = 0                  # records retained per signal; 0 keeps the viewer default
```

### `[migration]`

| Key | Default | Meaning |
| --- | --- | --- |
| `dir` | `"migrations"` | where migration files live, relative to the project |
| `auto` | `true` | apply pending migrations at the start of `pw dev` (dev loop only — never at application startup) |

### `[assets.tailwind]`

Present when scaffolded with Tailwind, absent otherwise.

```toml
[assets.tailwind]
enabled = true
input = "assets/app.css"
output = "public/generated/app.css"
minify = true
```

`input` and `output` must differ. `pw build` always minifies and `pw dev` never does; `minify` actually feeds `pw doctor`'s readiness finding. Leave it `true`. Tailwind plugins are `@plugin` declarations in the CSS entry, not keys here.

### `[assets.images]`, `[assets.css]`, `[assets.scripts]`

All default off; a project declaring none serves its authored tree verbatim.

| Key | Default | Meaning |
| --- | --- | --- |
| `assets.images.enabled` | `false` | convert `img src` PNG/JPG to WebP |
| `assets.images.quality` | `75` | lossy setting for JPEG re-encoding; PNG stays lossless |
| `assets.images.avif` | `false` | add an AVIF representation, chosen from `Accept` |
| `assets.css.minify` | `false` | minify stylesheets in place |
| `assets.scripts.enabled` | `false` | build a `.ts` entry point, minify authored `.js` |

### `[[packages]]` / `[package]`

An application lists each component package it links as `[[packages]]` with `module = "example.com/widget"` (must also be in `go.mod`). A `kind = "package"` project describes itself in `[package]` (`module`, `summary`, `requires.capabilities`, `migrations.dir`/`stem`/`engines`, `routes.register`, …). In a package, `generate.queries` must be empty.

## config.{APP_ENV}.toml (runtime config)

### Environment and file resolution

`APP_ENV` selects the environment: `dev`, `stg`, `prod`, or any token of lowercase letters, digits, `-`, `_`. Unset or empty means **`dev`**; an invalid token fails `ParseConfig`. `pw.Env()` returns the resolved token.

The loader reads the **first readable** candidate (files are never merged):

1. `./config.{APP_ENV}.toml`
2. `./config/config.{APP_ENV}.toml`
3. user configuration directory: environment-neutral `config.toml`
4. system configuration directory: likewise

A bare `config.toml` in the project tree is **never read**. `--config-path ./deploy/staging.toml` replaces the search entirely with no fallback. `pw.SetConfigLoadOptions` adjusts the search before `ParseConfig`.

Precedence, fixed: `default < TOML file < environment variable < command-line option`. An absent key in one layer never clears a lower layer's value.

### Deriving the three names of a key

Every struct field yields one stable key and two derived names. Take the TOML key, replace each `.` with `-`, prefix `--`: that is the option (`observability.query.slow_threshold` → `--observability-query-slow_threshold`). Take the option, drop the dashes, upcase: that is the environment variable (`OBSERVABILITY_QUERY_SLOW_THRESHOLD`). Underscores inside a key survive; only nesting dots change.

Deliberate exceptions: `server.port` answers to `PORT` / `--port`; `observability.service_name` to `OTEL_SERVICE_NAME`; `observability.otel.endpoint` / `.headers` to `OTEL_EXPORTER_OTLP_ENDPOINT` / `_HEADERS`; `auth.oidc.*` core keys to `AUTH_OIDC_*`. Three keys have no environment binding at all: `security.headers.content_security_policy`, its `_report_only` twin, and `security.headers.permissions_policy` — set them in TOML. The `[[middleware.rdb.connections]]` array is TOML-only (an array of tables has no flat name).

Durations are Go duration strings (`5s`, `200ms`, `24h`) — a bare number is rejected. Sizes are integers of bytes. Lists are TOML arrays, repeated CLI options, or comma-separated environment values.

### `${NAME}` environment expansion

Inside a TOML **string**, `${NAME}` expands from the process environment at load:

```toml
[[middleware.rdb.connections]]
group = "primary"
dsn = "postgres://app:${PRIMARY_DB_PASSWORD}@db1.internal:5432/app"
```

Rules: only strings expand (not keys, headers, numbers, booleans; array elements and `[[…]]` element fields do). An **undefined name fails the load** — an empty expansion would silently erase a default. A variable set to `""` counts as defined. `$$` yields a literal `$`. Expanded values still belong to the file layer for precedence. `${…}` in an environment or CLI value stays literal. No `${NAME:-default}` fallback form exists. This is the intended way to get credentials into array-of-tables elements, which have no option or variable of their own.

### `[server]` (selected keys)

| Key | Default | Meaning |
| --- | --- | --- |
| `port` | `8080` | HTTP listen port (`PORT`/`--port`) |
| `read_header_timeout` / `read_timeout` / `write_timeout` / `idle_timeout` / `shutdown_timeout` | `5s` / `30s` / `0s` / `2m` / `10s` | server timeouts; zero write timeout permits streams |
| `max_request_body` | `10485760` | bytes |
| `trusted_proxies` | `[]` | trusted proxy IPs/CIDRs |
| `health` / `readiness` / `openapi` | *(empty)* | endpoint paths; **empty serves nothing** — no default paths, deliberately |
| `api_doc` / `api_doc_path` | *(empty)* / `"/docs"` | `scalar`, `swagger`, or empty; requires `openapi` |
| `public.enabled` / `public.mount` / `public.read_local` | `true` / `"/public"` / `false` | embedded static assets |

An application route colliding with an enabled operational endpoint fails startup.

### `[middleware]` and `[[middleware.rdb.connections]]`

| Key | Default | Meaning |
| --- | --- | --- |
| `recovery` / `request_id` / `access_log` | `true` | standard stack |
| `compression` | `false` | zstd for HTML |
| `request_timeout` | `"0s"` | per-request deadline |
| `rdb.enabled` | `false` | open the framework-owned database pool |
| `rdb.default_group` / `rdb.write_group` / `rdb.migration_group` | *(empty)* | connection group selection |

Every database pool is one `[[middleware.rdb.connections]]` table; a reader-writer topology is several. The old `rdb.dsn` key is gone — an enabled database with no connections table fails at startup and names the replacement.

| Key | Default | Meaning |
| --- | --- | --- |
| `group` | *(empty)* | the name this connection is addressed by |
| `dsn` | *(empty)* | data source name; credential masked in reports |
| `readonly` | `false` | read-only transactions; never selected by `pw.SelectWriteDB` |
| `connect_timeout` | `"5s"` | per connection |
| `max_open_conns` / `max_idle_conns` | `0` | pool bounds |
| `conn_max_lifetime` / `conn_max_idle_time` | `"0s"` | |

### `[middleware.dynamo]`

Exists only when `github.com/shibukawa/popcornwave/database/dynamo` is imported. Keys: `enabled` (`false`), `region`, `endpoint` (empty selects the regional host; a value selects a local server), `access_key_id`, `secret_access_key`, `session_token` (redacted), `table_prefix`, `table_names` (`[]`), `timeout` (`"10s"`), `auto_migrate`. Declared-to-deployed table mapping: an explicit `[[middleware.dynamo.table_names]]` entry (`declared = "note"`, `deployed = "notes-prod-8f21c"`) wins; otherwise `table_prefix` applies; otherwise the declared name stands. A `table_names` entry naming a table no code declares is an error.

### `[middleware.firestore]`

Exists only when `database/firestore` is imported; Datastore mode required. Keys: `enabled` (`false`), `project_id` (falls back to `GOOGLE_CLOUD_PROJECT`, then `DATASTORE_PROJECT_ID`), `database`, `namespace`, `endpoint` (falls back to `DATASTORE_EMULATOR_HOST`), `credentials` (`"service_account"` | `metadata` | `oauth2` | `static`), `credentials_file` (falls back to `GOOGLE_APPLICATION_CREDENTIALS`), `timeout` (`"10s"`), `max_idle_conns` (`4`). `metadata`/`static` plus a `credentials_file` is an error.

### `[observability]`

Top level: `minimum_level` (`"info"`; `trace`…`off`), `stdout_format` (`"json"` or `plaintext`), `service_name`, `resource_attributes`, `boot_log` (`"auto"` | `tree` | `record` | `off`). Subsections:

- `[observability.query]` — statement logging: `enabled = "auto"` (on in dev), `slow_threshold = "200ms"` (zero disables slow detection, `explain`, and `reproduction` together), `bind_values = "auto"`, length bounds.
- `[observability.trace]` — framework spans: `enabled = "auto"` (follows whether traces are exported), `render`, `boundary`, `database`, `statement`. Bind values never reach a span.
- `[observability.otel]` — export: `enabled = false`, `endpoint` (OTLP/HTTP base; `/v1/traces` and `/v1/logs` appended), `headers`, `request_timeout = "10s"`, queue and batch bounds. `OTEL_EXPORTER_OTLP_TIMEOUT` is deliberately not bound to `request_timeout` (units differ).

### `[session]`

`enabled = false`; `backend = "rdb"` (`rdb` | `cookie` | `redis` | `dynamo` | `firestore`); `retention = "720h"`. Cookie keys: `cookie.name = "pw_session"`, `cookie.secure = true` (`false` refuses to start outside dev), `cookie.http_only = true`, `cookie.same_site = "lax"`. Backend-specific keys (`rdb.source`/`group`/`dsn`/`table`, `redis.dsn`/`key_prefix`, `dynamo.table`, `firestore.kind`) are read only for the selected backend, and a non-cookie backend needs its own blank import — the startup error quotes the line to add. `keyring.secret` is the base64 secret signing/sealing browser-carried data; required unless every registered slot is `Shared`/`RequestScope`. `pw init` generates one into `config.dev.toml`; other environments read `SESSION_KEYRING_SECRET`. Session lifetimes (`ttl`, `idle_timeout`, `renewal_interval`) live under `[auth]`, not here.

### `[security]` (brief)

Response headers are on by default: `headers.enabled = true`, `headers.content_type_options = true` (nosniff), `headers.frame_options = "deny"`, `headers.referrer_policy = "strict-origin-when-cross-origin"`. `headers.content_security_policy`, its `_report_only` twin, and `headers.permissions_policy` are empty by default and TOML-only. `headers.hsts.enabled = false`; HSTS is applied only on verified HTTPS requests. CSRF is off until you name the paths it covers (`security.csrf.enabled = false` in the scaffold); its secret is a session slot sealed by `session.keyring.secret`, not a key here.

### `[auth]` (brief)

Exists only when `plugin/auth` is linked (registering an account resolver does that). `enabled = false`; `backend = "rdb"`; `mode = "oidc_only"` (plus `jwt_only` for bearer-token APIs); `login_path`/`callback_path`/`logout_path`; `session.ttl = "24h"`; `protection.include`/`exclude`/`unauthenticated`. `[auth.oidc]`: `issuer`, `client_id`, `client_secret` (all `AUTH_OIDC_*`), `identity_claim = "sub"`, `admission = "authenticated"`. An enabled OIDC mode with empty issuer/client keys fails at startup naming both keys and variables — dev scaffolds carry no provider values because `pw dev` injects the emulator's. `[auth.jwt]` configures `jwt_only` (issuer, audience, algorithms, admission, revocation all required or bounded).

## Configuration declarations (application code)

Declare a struct, register it under a prefix, read it from the request context. `pw generate` reads the registration call (the file must be in a `generate.config` directory) and writes the binding — no runtime reflection, TinyGo-safe.

```go
type AppConfig struct {
	EnvLabel      string `default:"local" help:"environment name shown in the page badge"`
	EnvLabelColor string `default:"#64748b" help:"CSS color of the environment badge"`
}

func RegisterConfig() { pw.RegisterConfig[AppConfig]("app") }
```

Call `RegisterConfig()` from `main` **after** every package `init` has run and **before** parsing begins (registering after `ParseConfig` panics). The prefix must be a string literal; prefixes share one namespace and may contain dots (`"middleware.cache"`). Read anywhere a request context is:

```go
app := pw.Config[AppConfig](r.Context())
```

No error return: an unparsed prefix yields declared defaults, an unregistered type yields the zero value, `nil` is accepted outside a request.

With prefix `app`, field `Mailer.FromAddress` becomes stable key `app.mailer.from_address`, TOML `[app.mailer] from_address = …`, option `--app-mailer-from_address`, variable `APP_MAILER_FROM_ADDRESS`.

**Bindable field types**: `string`, `bool`, `int`, `time.Duration` (duration-string syntax in every source, including `default`), `[]string`, named nested structs of those, and `[]T` where `T` is a named struct in the same package (filled from an array of tables — file-only, no CLI/env per element). Floats, maps, pointers, and other slice element types are **not** bindable; declare `string`/`int` and convert after parsing.

**Tags**: `default:"value"`, `key:"name"`, `opt:"long"` or `opt:"long,s"` (also moves the env name, which derives from the long option), `env:"NAME"` or `env:"-"`, `help:"text"` (falls back to the field's godoc, written back into the tag on first generate). Presentation tags: `secret:"mask"|"hide"|"show"` (auto-masking already triggers on key paths containing `password`, `secret`, `token`, `dsn`, etc.), `dependon:".sibling"` (hides dependents in the startup summary while the parent is off — display only, binding is unaffected), `falsy:"value"` (teaches `dependon` what "off" means for strings, ints, durations; a numeric/duration `dependon` parent without `falsy` fails generation).

**Scaffolds** — the binary is the authoritative key list for its own build:

```sh
./myapp --generate-config toml > config.dev.toml
./myapp --generate-config env > .env
```

Both exit after writing. Parsing reads process environment variables, never a `.env` file — a dotenv loader or the shell must export it.

**Command-line forms** accepted by the binary:

```sh
./myapp --app-mailer-from_address noreply@example.com
./myapp --app-mailer-from_address=noreply@example.com
./myapp --app-tls-enabled              # a bool with no value is true
./myapp --app-tls-enabled=false
./myapp --app-origins a.example --app-origins b.example   # []string accumulates
```

An unknown option, a missing value, and an invalid boolean all fail the load.

**CLI-only subcommands**: `pw.RegisterSubCommand[MigrateOptions]("migrate", "run database migrations")` with `arg:"required"|"optional"|"*"` tags for positionals; read with `options, ok := pw.Command[MigrateOptions]()`. Subcommand structs read no TOML and no environment.

**Startup summary**: resolved configuration is reported once at startup (tree on a terminal, one structured record elsewhere), each entry with the source its value won from. `secret` controls disclosure (`dsn`-suffixed keys keep scheme/host/port and mask only user info), `dependon` filters disabled branches, `observability.boot_log` selects the format.

## Common mistakes

- Putting a runtime table (`server`, `session`, `middleware`, `observability`, a DSN) in `popcornwave.toml`, or a `[generate]`/`[dev.watch]` table in `config.*.toml`. The split is enforced; the loader errors.
- Expecting a bare `config.toml` in the project tree to be read — it never is. The project-local file is always `config.{APP_ENV}.toml`.
- Expecting file merging: only the first readable candidate is read. `config.prod.toml` must be a complete configuration, not a diff over `config.dev.toml`.
- Writing a bare number for a duration (`send_timeout = 5`) — rejected in every source, including `default` tags.
- `${NAME}` naming a variable the process does not have — the load fails (by design; check the deployment environment).
- Registering config after `ParseConfig`, or with a computed (non-literal) prefix, or from a file outside `generate.config` — panics, no binding, or nothing generated respectively.
- Declaring a `float64`, map, or pointer field — generation error; bind a supported type and convert.
- Relying on a TOML typo being caught: an unknown TOML key parses, matches no field, and is silently not applied (unlike a misspelled CLI option, which fails loudly).
- Setting a key and not finding it in the startup summary — check its `dependon` parent, not your spelling; the value was still bound.
- Trying to set a `[[middleware.rdb.connections]]` element via environment variable — array elements are file-only; use `${NAME}` expansion inside the DSN string instead.
