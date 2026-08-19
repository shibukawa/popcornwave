---
name: popcornwave
description: >
  Guidelines for working in a Popcorn Wave (pw) Go web project. Load this when
  creating or editing .pw.html templates, .pw.sql / .pw.dynamo / .pw.firestore
  queries, handlers, pages, popcornwave.toml, or config.*.toml; when adding
  routes, database access, sessions, or auth; or when running, building,
  checking, or debugging a project with the pw command (pw dev, pw generate,
  pw build, pw doctor, pw migrate, pw fmt); or when investigating structured
  development logs in .log with DuckDB.
---

# Popcorn Wave

Popcorn Wave is a Go web framework built around code generation: you author
typed template sources (`.pw.html`), typed query sources (`.pw.sql`,
`.pw.dynamo`, `.pw.firestore`), Go handlers, and TOML configuration; the `pw`
command generates the Go that glues them together. Routing stays on
`pw.ServeMux`, a type alias compatible with `net/http.ServeMux`.

## Hard rules

1. **Never edit `*_pw_gen.go` files.** They are build outputs. Change the
   source (`.pw.html`, `.pw.sql`, handler signatures, config declarations) and
   run `pw generate` to rebuild them. They are gitignored in applications and
   committed in package projects — either way the generator owns them.
2. **A directory is invisible to generation until it is listed.** The
   `[generate]` table in `popcornwave.toml` lists source directories per
   purpose (`handlers`, `templates`, `queries`, `config`, `pages`, `dynamo`,
   `firestore`). Creating a new source directory without adding it there
   silently generates nothing.
3. **`.pw.html` is a typed template language**, not Go `html/template`.
   `.pw.sql` is a typed SQL template, not a string with placeholders. Do not
   guess syntax — read [references/templates.md](references/templates.md) and
   [references/sql.md](references/sql.md) before writing either.
4. **Prefer `pw new` over hand-scaffolding** a handler or page: it writes the
   route, the template, and the registration in the shape generation expects.
5. **Configuration is per-environment.** `config.dev.toml` is development;
   deployed environments read `config.prod.toml` (selected by `APP_ENV`) and
   pass secrets as `${ENV_VAR}` references, never literals.

## Check loop — run after every change

Run these from the project root (inside `devbox shell` if the project has
`devbox.json`), in this order — `pw fmt` rewrites sources, so it comes before
`pw generate`, or doctor reports the generated files as stale (PW0111):

```bash
pw fmt             # rewrite template/query sources into canonical form
pw generate        # rebuild *_pw_gen.go from sources; fails on syntax/type errors
go build ./...     # the generated code and your Go compile together
go test ./...      # unit tests
pw doctor          # configuration and project health report
```

- `pw fmt --check` and `pw generate --check` verify without writing (CI, and
  package projects whose artifacts are committed).
- `pw build` is the full pipeline: generate, build assets, compile the binary.
  Use it as the final gate for asset-touching changes.
- `pw doctor --env=prod` (or any config token) reports what that environment
  will actually run — use it after editing `config.<env>.toml`. `--online`
  additionally contacts the database and reads migration state.
- Template/query type errors surface in `pw generate`; route conflicts and
  config mistakes surface in `pw doctor` and at startup.

Gotcha: a project scaffolded by an older `pw` release can fail `pw fmt --check`
before anyone edited anything — early scaffolds were not written in canonical
form. Run `pw fmt` once and commit; current releases scaffold sources that
already pass.

## Running the app

`pw dev` watches sources, regenerates, rebuilds, restarts, and starts the
development services the project declares (database server, Valkey,
dynamodb-local, dev identity provider). The startup summary prints the port
and every mounted route. Development conveniences (dev console, relaxed auth,
seeded logins) exist only under `pw dev` — never replicate them in handlers.

For database schema changes: write a new `migrations/NNNNN_name.sql` (never
edit an applied one), then `pw migrate up` — or let `pw dev` apply it in
development. `pw migrate status` shows where you are.

## Where to read next

| Task | Reference |
| --- | --- |
| Project layout, generation model, routers, build pipeline | [references/architecture.md](references/architecture.md) |
| Writing `.pw.html` templates (syntax, components, slots, types) | [references/templates.md](references/templates.md) |
| Page trees (discovered routing), async/partial/live rendering, forms | [references/rendering.md](references/rendering.md) |
| Handlers, request binding, responses, middlewares, sessions, auth | [references/handlers.md](references/handlers.md) |
| `.pw.sql` queries, migrations, seed data | [references/sql.md](references/sql.md) |
| DynamoDB and Firestore stores | [references/dynamo-firestore.md](references/dynamo-firestore.md) |
| `popcornwave.toml`, `config.<env>.toml`, config declarations | [references/config.md](references/config.md) |
| Local JSONL logs, trace correlation, and DuckDB analysis | [references/telemetry.md](references/telemetry.md) |
| pw dev, testing, e2e, API docs, diagnostics | [references/workflow.md](references/workflow.md) |

## pw command summary

| Command | Purpose |
| --- | --- |
| `pw init` | create a project in a new directory (wizard, or `--yes` + flags) |
| `pw add <capability>` | enable a declined capability (database, auth, tailwind, …) later |
| `pw new [handler\|page]` | scaffold a handler or page beside the ones you have |
| `pw generate [--check]` | regenerate everything derived from your sources |
| `pw fmt [--check] [<path>…]` | format template sources into canonical form |
| `pw migrate <action>` | inspect and apply database migrations |
| `pw seed [<name>…]` | load seed datasets into the database |
| `pw prepare` | generate and build assets, stopping before the compiler |
| `pw build` | generate, build assets, and compile the project |
| `pw dev` | watch, regenerate, rebuild, restart, and run dev services |
| `pw doctor [--env=…] [--strict]` | report what an environment will actually run |

Documentation: https://shibukawa.github.io/popcornwave/
