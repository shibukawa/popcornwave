# Architecture

How a Popcorn Wave project is laid out, what `pw generate` reads and writes, how the two routers coexist, and how the build pipeline turns sources into a binary. Everything here describes a project created by `pw init`; directory names are the scaffold defaults, and every consumer reads `popcornwave.toml` rather than the names, so a renamed directory only needs its `[generate]` entry updated.

## Scaffolded project layout

```
myapp/
├── popcornwave.toml        # build config, owned by the pw command — never runtime settings
├── config.dev.toml         # runtime config for APP_ENV=dev (default env)
├── config.prod.toml        # runtime config for APP_ENV=prod
├── go.mod
├── cmd/myapp/main.go       # entry point; pw.Run, config registration calls
├── handlers/               # registered routing: mux + handler Go files + page templates beside them
│   ├── index.go            # owns the package mux; func Handlers() returns it
│   ├── home_handler.go
│   └── home.pw.html
├── pages/                  # discovered routing: a directory holding page.pw.html is a route
│   ├── layout.pw.html
│   ├── page.pw.html        # GET /
│   └── greet/name_/        # GET /greet/{name}
│       ├── page.pw.html
│       └── page.go         # optional Load() between request and render
├── templates/              # the one document shell + error pages, shared by both routers
│   ├── document.pw.html
│   ├── templates.go
│   ├── errors.go
│   └── 400|401|403|404|409|413|500.pw.html
├── queries/                # .pw.sql sources (present when the project uses a database)
│   └── users.pw.sql
├── migrations/             # one ordered migration stream for the whole application
│   └── 00001_init.sql
├── public/                 # static assets as you author them
│   └── app.css
├── public.go               # package publicassets; go:embed all:dist/public (hand-owned, not generated)
├── dist/                   # build output: dist/public is the built asset tree that ships
│   └── public/.keep        # sentinel so go:embed works before the first build
├── devbox.json             # pinned toolchain + dev services (optional)
├── .vscode/settings.json   # hides *_pw_gen.go from the editor
└── .gitignore              # ignores *_pw_gen.go, dist/*, the binary
```

Notes on ownership:

- `handlers/` and `pages/` are both optional — a project carries either router or both, chosen at `pw init`.
- `templates/` always holds the document shell and the error templates; both routers render through them. Exactly one `document.pw.html` may exist in the whole tree.
- `dist/` is entirely build output (`dist/public`, a conversion cache, a manifest); `.gitignore` excludes everything under it except `dist/public/.keep`.
- `public.go` is ordinary application code you may edit; the manifest beside it (`public_manifest_pw_gen.go`) is generated.
- The compiled binary lands in the project root, named after the main package.

### Where does X go

| You are adding… | Put it in | Then |
| --- | --- | --- |
| A registered route + handler | `handlers/*.go`, register on the package mux in `init()` | nothing else — `pw dev` regenerates |
| A page template used by a handler | same directory as the handler (`handlers/`) | directory must be listed under `generate.templates` |
| A discovered-routing page | `pages/<segment>/page.pw.html` | directory name is the route; `name_/` = `{name}`, `rest__/` = `{rest...}` |
| Go logic for a page | `page.go` beside `page.pw.html`, `func Load(...)` | signature decides the rung (typed vs raw handler) |
| A typed SQL query | `queries/*.pw.sql` | directory must be listed under `generate.queries` |
| A schema change | `migrations/NNNNN_name.sql` | `pw dev` applies it (unless `migration.auto = false`) |
| A static file | `public/…` | served at `/public/…` after a build copies it to `dist/public` |
| A shared layout / alternative shell | exported component with an unnamed slot, selected via `pw.WriteHTMLChain` | never a second `document.pw.html` |
| A typed config struct | any package listed under `generate.config` (scaffold: `cmd/myapp`) | see references/config.md |
| A new top-level source directory | anywhere | **must be added to the relevant `[generate]` purpose** or it is invisible |

## The generation model: purposes

`pw generate` is scoped **per purpose**. Each purpose in `popcornwave.toml` lists the directories it may read, and reads nothing else. A directory is invisible to a purpose until it is listed.

```toml
[generate]
handlers = ["handlers"]
templates = ["handlers", "templates"]
queries = ["queries"]
config = ["cmd/myapp"]
pages = ["pages"]
dynamo = []
firestore = []
```

| Purpose | Reads | Generates |
| --- | --- | --- |
| `handlers` | route registrations, `pw.Parse` and response calls in Go | request binding, JSON codecs, the OpenAPI fragment |
| `templates` | `.pw.html` | typed renderers; also where the document shell and error pages are found |
| `queries` | `.pw.sql` | context-based query functions |
| `config` | `pw.RegisterConfig` and `pw.RegisterSubCommand` calls in Go | configuration and subcommand binding |
| `pages` | page-tree roots | route registration and page parameters for discovered routing |
| `dynamo` | `dynamo`-tagged Go types and `.pw.dynamo` | record codecs, keys, typed DynamoDB queries |
| `firestore` | `firestore`-tagged Go types and `.pw.firestore` | entity codecs, keys, typed Datastore-mode queries |

Rules that matter in practice:

- `handlers`, `templates`, `queries`, and `config` are **required with no default**; a missing key is an error, and `[]` is the explicit way to say "this project generates nothing for this purpose". `pages`, `dynamo`, and `firestore` are optional.
- A directory may appear under several purposes. The scaffold lists `handlers` under both `handlers` and `templates` because a page template lives beside the handler that renders it.
- Each listed directory is walked recursively — `webroot/admin/queries` is covered by a `webroot` entry. Only a new **top-level** source directory needs an edit.
- No duplicates, and no entry nested inside another entry of the same purpose.
- A `.pw.html` or `.pw.sql` outside the purpose that owns it is reported and skipped, not silently picked up: `pw: samples/home.pw.html is outside generate.templates and is not generated from; list its directory to include it`
- Go sources are never reported this way — a call site outside its purpose simply gets no generated binding.
- Exactly one `generate.templates` entry holds the document shell; a second `document.pw.html` anywhere fails generation.
- A `generate.pages` entry is a whole tree and must not be listed under (or nested with) `templates` or `handlers`.

Besides declaration files, generation reads Go source for call sites: `pw.Parse[T]`, `pw.WriteAPI[T]`, `pw.NewStream[T]`, `pw.RegisterConfig[T]`, `pw.RegisterSubCommand[T]`, and the error constructors (`pw.BadRequest` etc.). The same evidence feeds one OpenAPI 3.1 fragment per package, merged deterministically at build time.

## `_pw_gen.go` files

Generated Go is **build output, never source**:

- Filenames are `{source-base}_pw_gen.go`, always written beside the source.
- They are excluded from version control by the scaffolded `.gitignore` and hidden from the editor by `.vscode/settings.json`.
- They are recreated on every application build. **Never edit them, never commit them** — regenerate with `pw generate`.
- A `_pw_gen.go` left outside every purpose by an earlier layout IS reported, because nothing regenerates or removes it anymore. Delete it.

One exception in kind, not in rule: `cmd/<name>/popcornwave_bootstrap_pw_gen.go` is a generated file of blank imports that links the document shell and the embedded public assets into the binary, so no handler has to reference them. It is removed automatically when neither exists. Still generated, still not yours to edit.

## Two routers, one mux

Popcorn Wave has **registered routing** (routes you write in Go) and **discovered routing** (routes derived from a `pages/` directory tree). They coexist on one mux, and a project can carry either or both.

| | Registered router | Discovered router |
| --- | --- | --- |
| Where a route comes from | a registration you wrote in Go | a directory holding `page.pw.html` |
| Methods | any | `GET` for pages, `POST` for actions |
| Inputs | path, query, headers, cookies, body, multipart | path and query |
| Generated OpenAPI | yes | no, by design |
| Fails when | a pattern is not a compile-time constant | a directory name is not a legal Go package name |

Registered routing is the scaffolded `handlers` package: a package-owned mux, routes registered in `init()`:

```go
// handlers/index.go
package handlers

import "github.com/shibukawa/popcornwave/pw"

var mux = pw.NewServeMux()

func Handlers() *pw.ServeMux { return mux }
```

```go
// handlers/home_handler.go
func init() { mux.HandleFunc("GET /dashboard", dashboard) }
```

`pw.ServeMux` is a **type alias**: `pw.NewServeMux` returns `*http.ServeMux` on ordinary Go builds. Patterns, wildcards, and precedence are exactly the standard library's. A separate implementation with the same semantics is compiled in only for TinyGo. Non-TinyGo scaffolds may use `http.NewServeMux()` directly — the two are the same type there.

Discovered routing: `pw generate` walks the `pages` tree and writes the registrations. One trailing underscore in a directory name is a dynamic segment, two are a catch-all (`pages/users/id_/` → `GET /users/{id}`, `pages/files/rest__/` → `GET /files/{rest...}`). Bracket spellings like `[id]` are impossible because the directory is also a Go package. The root page registers `GET /{$}`, not `GET /`. A page alone is a generated handler; adding `page.go` with `func Load(id string) (User, error)` puts typed Go between decode and render; a `Load(w http.ResponseWriter, r *http.Request)` takes over the response entirely.

Both share one mux without negotiating:

```go
mux := handlers.Handlers()  // registered: your API
pages.Register(mux)         // discovered: the website
```

Registration order does not matter. The one real collision: registering the same method and path twice panics at startup (standard library behavior).

Use the discovered router for `GET` pages and their `POST` actions. A file download, a webhook, a `PUT`, anything that must appear in OpenAPI — those are registered routes.

Larger apps compose registered areas the plain `net/http` way: each area package owns a mux and exports `Handlers()`, the parent mounts children with `mux.Handle("/admin/", http.StripPrefix("/admin", admin.Handlers()))`. The parent imports children (never the reverse), so children's `init` registrations run first and no import cycle forms. Avoid mounting an area at `/public/` — it collides with the embedded asset mount (`server.public.mount`, default `/public`).

## The standard-library boundary

Adopting the framework does not teach the codebase a private request model:

- Handlers are `http.HandlerFunc`; `w` and `r` are the standard types, testable with `httptest`.
- Middleware is `func(http.Handler) http.Handler`.
- `pw.Run(ctx, handler)` is a convenience wrapper over `pw.Middlewares` and `http.Server`. `pw.Middlewares` hands back a plain `http.Handler` when you want to own the server yourself.
- The framework proper is the middleware stack (request ID, recovery, body limit, security headers, timeout, access log, assets, OpenTelemetry), startup validation, the database pool, and the operational endpoints. `pw.Parse[T]`, `.pw.sql` statements, `.pw.html` components, and `pw.WriteAPI`/`WriteHTML`/`NewStream`/`WriteProblem` are replaceable helpers — usable from another framework, or replaceable with hand-written code per endpoint.

The generated layers come from `tinybind-go`, wrapped behind the single stable `pw` package:

| Layer | Responsibility |
| --- | --- |
| `httpbind` | request binding, validation, JSON and streaming responses, OpenAPI |
| `htmlbind` | typed HTML components and render chains |
| `sqlbind` | typed SQL statements and result scanning |
| `configbind` | configuration binding, scaffolds, subcommands |

### The request path

A request passes a fixed chain before your handler; each stage depends on the one above:

| Stage | What it decides |
| --- | --- |
| Operational endpoints | health, readiness, framework assets answer above everything |
| Public assets | a static file is served without touching a route |
| Body limit | an oversized body is refused before it is read |
| Security headers | the response policy the browser will apply |
| Recovery | a panic becomes a response instead of a dropped connection |
| Session | the cookie becomes a validated record, or the request is anonymous |
| Authentication | the login endpoints answer their own paths |
| CSRF | an unsafe request proves it came from your page |
| Guard | an unauthenticated request to a protected path is redirected |
| Your handler | — |

Login endpoints sit above CSRF, so a login POST and an OIDC callback never need a token.

## The pw command

| Command | Purpose |
| --- | --- |
| `pw init` | create a runnable project in a new directory |
| `pw add` | install a capability the project declined at init (database, dynamo, firestore, images, …) |
| `pw new` | scaffold one more handler, route, and template |
| `pw generate` | compile `.pw.html`, `.pw.sql`, page trees, and call sites into Go |
| `pw dev` | watch, regenerate, migrate, rebuild, restart |
| `pw prepare` | everything a build needs, stopping before the compiler |
| `pw build` | produce a release binary |
| `pw migrate` / `pw seed` | inspect/apply migrations; load seed datasets |
| `pw doctor` | resolve a named environment (`--env=prod`) and report findings with stable `PW0xxx` identifiers |

Do not confuse `pw` with the deployed binary's own command line, which carries configuration flags, `--generate-config`, and application-defined subcommands.

## Build pipeline

Generation runs before compilation, always:

- **`pw generate [--check]`** — compiles templates, SQL, page trees, and call sites into `_pw_gen.go` files beside their sources. `--check` writes nothing and exits non-zero listing stale files; use it in CI, since gitignored output can't show staleness in a diff.
- **`pw dev`** — the everyday loop. On startup: starts Devbox services, runs `pw generate`, applies pending migrations (unless `migration.auto = false`), builds Tailwind CSS unminified and starts its watcher, starts the dev identity provider and telemetry viewer if configured, then builds and runs `project.main`. It then polls watched files twice a second and repeats only the affected steps. It watches the whole module (any Go source is a rebuild input), wider than the `[generate]` purposes; trim with `dev.watch.excludes`, extend with `dev.watch.includes`. In dev, a taken `server.port` shifts to the next free one (up to ten) with a warning; every other `APP_ENV` binds strictly.
- **`pw build`** — release binary: runs `pw generate`, builds Tailwind **minified** (overriding `assets.tailwind.minify`), builds the asset tree into `dist/public` (conversions, `.zstd` sidecars, cache manifest), rejects the build if `project.main` depends on a development-only package (`contrib/devidp`), then runs `go build` on `project.main`. Cross-compile with the usual `GOOS`/`GOARCH` env vars.
- **`pw prepare`** — `pw build` without the final compile step. Use it before invoking a different compiler yourself.

Both `pw dev` and `pw build` generate first, so direct `pw generate` invocation is mostly for CI and for diagnosing generation errors. Every `pw` command except `init` and `version` locates the project by walking up to `popcornwave.toml`, so it works from any subdirectory.

## Embedded public assets

`public/` is what you author; the binary embeds `dist/public`, the built tree:

```go
// public.go — ordinary application code, not generated
package publicassets

//go:embed all:dist/public
var embeddedPublic embed.FS

func init() {
	middlewares.RegisterPublicFS(PublicFS())
}
```

The `init` registration is why `main` never mentions assets: the framework mounts the filesystem at `server.public.mount` (default `/public`) during startup. `pw build` copies or transforms `public/` into `dist/public` (byte-for-byte when no conversions are enabled) and writes a generated manifest (`public_manifest_pw_gen.go`) that decides every cache header ahead of time. A URL the build did not declare is a 404 regardless of what the tree holds. Files keeping their authored name get `public, no-cache` + strong `ETag`; files the build produced are digest-named and get `immutable`. `dist/public/.keep` exists only so `go:embed` does not fail before the first build; the build replaces the tree.

## TinyGo

The generated path uses no runtime reflection, which is what makes TinyGo a first-class target. `pw build` always links with host `go`; a TinyGo build runs the preparation and then the compiler itself:

```sh
pw prepare
tinygo build -scheduler=threads -o myapp ./cmd/myapp
```

Use `pw prepare`, not bare `pw generate` — generation alone does not build `dist/public`, and `go:embed` fails on a tree that was never built. `-scheduler=threads` is required for network-protocol database engines; `database/postgres` and `database/mysql` refuse to compile without it. TinyGo scaffolds include a root `tinygohelper.go` (`//go:build tinygo`) that blank-imports `github.com/shibukawa/tinygodriver/netdev`; without it a TinyGo binary exits at startup with `Netdev not set`. Projects created `--no-tinygo` lack the file — add it before switching. On TinyGo, `pw.NewServeMux` compiles in the framework's own ServeMux implementation with Go 1.22 pattern semantics, since TinyGo's does not support them.

## Common mistakes

- Editing a `_pw_gen.go` file. It is build output; the next `pw generate` (or `pw dev`/`pw build`, which run it) overwrites your change. Edit the source (`.pw.html`, `.pw.sql`, the call site) instead.
- Creating a new top-level source directory and forgetting to list it under the right `[generate]` purpose — its sources are invisible, and `.pw.*` files there are reported and skipped.
- Adding a second `document.pw.html` for an area. Generation fails; use an exported component with an unnamed slot plus `pw.WriteHTMLChain`.
- Spelling a dynamic page directory `[id]` instead of `id_` — it breaks `go build ./...` for the whole module.
- Mounting a registered area at `/public/`, colliding with the embedded asset mount.
- Running `pw generate` then `tinygo build` directly — use `pw prepare` so `dist/public` exists for `go:embed`.
- Putting runtime settings (ports, DSNs, cookies) into `popcornwave.toml` — the loader rejects them; they belong in `config.{APP_ENV}.toml` (see references/config.md).
