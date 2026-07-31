---
title: pw init
description: Create a runnable Popcorn Wave project.
sidebar:
  order: 1
---

```sh
pw init <project-name> [--tailwind] [--no-devbox] [--no-database] [--no-redis] [--auth=<mode>] [--session=<backend>] [--devidp]
```

The command creates a complete, runnable project in a new directory. A name and
flags make the operation non-interactive; omitting the name, or passing `-i`,
presents the same choices as a wizard.

## Options

| Option | Effect |
| --- | --- |
| `--tailwind` | also scaffold the Tailwind CSS toolchain |
| `--no-tinygo` | target host Go instead of TinyGo |
| `--no-devbox` | no `devbox.json`; keep your own setup — mise, Docker Compose, Nix, Homebrew, Scoop |
| `--no-database` | no rdb configuration, no migrations, and no SQL example |
| `--no-redis` | leave the Valkey development server out of `devbox.json` |
| `--auth=<mode>` | `none` (default), `oidc`, `oidc-passkey`, or `passkey` |
| `--session=<backend>` | with a login, where sessions live: `rdb` (default), `cookie`, or `redis` |
| `--devidp` | with an OIDC mode, wire up the local identity provider |
| `-i`, `--interactive` | ask every question even when a name was given |

`--tailwind`, `--no-database`, `--no-redis`, and `--auth` all select
capabilities [`pw add`](/pw/project/add/) can install later, so declining one
costs nothing permanent. The database is the exception in one direction only:
the login ceremony and admission tables live in it whichever backend stores the
sessions, so `--no-database` with an `--auth` mode is rejected, and the wizard
skips the authentication question entirely when the database is declined.

`--no-tinygo` is the answer `pw add` cannot revisit — see
[Changing the toolchain](#changing-the-toolchain).

## Changing the toolchain

The selected compiler is recorded as `project.toolchain` in `popcornwave.toml`,
and it decides which mux type the handler packages use: TinyGo projects route
through `pw.ServeMux` so one import works on both toolchains, host-only projects
keep `http.ServeMux`. Generation discovers either, so the difference is confined
to the scaffold.

There is no command for switching afterwards, because the change reaches source
you own. Doing it by hand is four edits:

1. set `project.toolchain` in `popcornwave.toml` to `tinygo` or `go`;
2. swap the mux type and accessor in each handler package's `index.go`;
3. add or remove `tinygo@latest` in `devbox.json`;
4. add or remove `tinygohelper.go`, the TinyGo-only netdev registration —
   without it a TinyGo binary aborts at startup with `Netdev not set`.

Then run [`pw generate`](/pw/project/generate/). A `project.toolchain` value
other than `tinygo` or `go` is rejected when the project loads.

## Authentication

Authentication changes more than one flag, so the selected mode determines the
`[auth]` section written to `config.dev.toml`:

| Answer | `auth.mode` | What it means |
| --- | --- | --- |
| None | — | no `[auth]` section at all |
| OIDC | `oidc` | every login goes through an OpenID Provider |
| OIDC + passkey | `oidc_passkey` | OIDC bootstraps the account, passkeys handle repeat logins |
| Passkey only | `passkey_only` | no external provider; recovery policy is yours |

Choosing an OIDC mode asks one follow-up: **local emulator** or **external
provider**.

The local emulator is the development identity provider that
[`pw dev`](/pw/project/dev/) runs. `pw init` sets `dev.idp.enabled` in
`popcornwave.toml` and writes a `devidp.toml` roster with two starter users, and
`pw dev` injects the issuer and generated client credentials — so nothing about
the provider is written into a committed config file.

For an external provider, `config.dev.toml` contains empty `issuer`,
`client_id`, and `client_secret` values. Those values are placeholders, **not
optional settings**. The application refuses to start until they are supplied
in the file or through `AUTH_OIDC_*` environment variables; the remaining
alternative is to use the emulator.

## Session storage

An `--auth` mode asks one more question: where a login session lives. All three
answers read the same in a handler — `session.Read[T]` and the auth helpers do
not change — so this is a deployment decision, not an API one.

| Answer | `session.backend` | What the project gets |
| --- | --- | --- |
| Database | `rdb` | one row per session, revocable, swept; carries a migration |
| Cookie | `cookie` | the record sealed into a second cookie; no storage, no revoking |
| Redis or Valkey | `redis` | server-side TTL per record; revocable, nothing to sweep |

**Storage is opt-in by blank import.** A session backend registers itself from
its package `init`, so the one line that imports it is what puts it — and its
client library — in the binary:

```go
// cmd/myapp/main.go, written by pw init
import _ "github.com/shibukawa/popcornwave/plugin/session/rdb"
```

`pw init` writes that line for `rdb` and `redis`. The cookie backend is built
into `pw` and needs none, which is why a project can start with sessions and no
storage at all. A project on `rdb` never links the Redis client, and the reverse
holds too.

Configure a backend without its import and startup stops with the missing line
quoted, rather than with a login that fails at the first request:

```
session.backend = "redis" needs its plugin; add to the application:
import _ "github.com/shibukawa/popcornwave/plugin/session/redis"
```

The answer also decides what else is scaffolded. `rdb` writes the session table
migration; `cookie` and `redis` own no table, so the auth migration takes the
free version instead. `redis` adds the Valkey development server to
`devbox.json` even if `--no-redis` was passed, because the session it configures
needs a server to reach. `cookie` writes
`cookie_store.secret = "${SESSION_COOKIE_SECRET}"` and the command prints the
line that generates one:

```sh
export SESSION_COOKIE_SECRET=$(openssl rand -base64 32)
```

[Cookies](/guides/cookies/) compares the three in terms of revocation, size, and
who enforces expiry.

## Validation

The project name accepts letters, digits, `-`, and `_`; `.` and `..` are
rejected. The destination must be empty or absent — an accidental `pw init .`
in a populated tree fails rather than scattering files.

## What it writes

```
myapp/
├── popcornwave.toml           project name, main package, generation sources
├── config.dev.toml            runtime configuration for APP_ENV=dev
├── go.mod
├── devbox.json / devbox.lock  Go + Valkey (+ tailwindcss with --tailwind)
├── cmd/myapp/main.go          calls pw.Run
├── handlers/
│   ├── index.go               the package-level mux and Handlers()
│   ├── home_handler.go        route registration and the net/http handler
│   └── home.pw.html           typed page template
├── templates/
│   ├── document.pw.html       shared document shell
│   ├── templates.go           package marker, present before first generation
│   └── 400|404|500.pw.html    error pages
├── queries/users.pw.sql       named SQL with a typed result (with the database)
├── migrations/00001_init.sql  initial schema, in goose format (with the database)
│                              plus framework tables with a login: the session
│                              table only for --session=rdb
├── public/.keep               empty-tree sentinel; never served
├── public.go                  embeds public/ and registers it
├── .vscode/settings.json      hides **/*_pw_gen.go
└── .gitignore                 excludes *_pw_gen.go and other build output
```

`popcornwave.toml` names the directories it just created under each `[generate]`
purpose, because [`pw generate`](/pw/project/generate/) reads those lists and
has no default to fall back on. `handlers` appears under both `handlers` and
`templates`, since the starter page template sits beside its handler, and
`cmd/myapp` appears under `config`, where the application registers its
configuration.

With an OIDC mode plus `--devidp` it also writes `devidp.toml`, the roster of
selectable development users, and adds `[dev.idp]` to `popcornwave.toml`.

With `--tailwind` it also writes `assets/app.css` and
`public/generated/app.css`, adds the `[assets.tailwind]` block to
`popcornwave.toml`, pins `tailwindcss` in `devbox.json`, and links the
stylesheet from the document shell. No `package.json` and no Node lockfile are
created. See [Styling](/guides/styling/) for enabling this later.

Each file is written to a temporary path and renamed into place. If the command
is interrupted, it cannot leave a half-written source file behind.

## What it runs

Writing files is not enough to prove the scaffold is usable. `pw init` therefore
runs `go mod tidy` and [`pw generate`](/pw/project/generate/) before reporting
success, leaving a project that compiles immediately:

```
Created myapp

Not included: redis-valkey, auth, tailwind
  pw add <capability> enables one later

  cd myapp
  devbox shell
  pw dev
```

The notice lists only what this run declined, so a scripted `pw init` learns the
same thing the wizard says beside each answer.

Declining Devbox drops the `devbox shell` line, since there is no shell to
enter. Tailwind then has no pinned toolchain either, so the scaffold states what
to install:

```
Tailwind CSS needs its own toolchain here:
  install the standalone tailwindcss CLI, version 4 or later
```

It names the requirement rather than the Devbox package, because
`tailwindcss_4@4.1.18` is a nixpkgs identifier that means nothing to mise,
Homebrew, or Scoop. [`pw build`](/pw/project/build/) reports the same when the
binary is missing.

The generated `go.mod` requires the framework at the version of the `pw` binary
that created it. When `pw` was built from a working copy rather than a release,
it writes a `replace` directive pointing at that checkout instead.

## Next steps

- [Getting started](/start/getting-started/) — a walkthrough of the output.
- [pw add](/pw/project/add/) — installing a capability you declined here.
- [pw new](/pw/project/new/) — adding the second handler.
- [Project structure](/guides/project-structure/) — growing past one package.
