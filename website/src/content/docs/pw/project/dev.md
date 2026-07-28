---
title: pw dev
description: The development loop — services, generation, migrations, CSS, and restart on change.
sidebar:
  order: 5
---

```sh
pw dev
```

This is the everyday command. It takes no arguments because the project file
defines the loop.

## What it does on startup

1. starts the Devbox services declared in `devbox.json`;
2. runs [`pw generate`](/pw/project/generate/);
3. applies pending migrations, unless `migration.auto` is `false`;
4. builds the Tailwind stylesheet and starts its watcher, if Tailwind is enabled;
5. starts the development identity provider, if `dev.idp.enabled` is `true`;
6. builds and runs `project.main`.

After startup, it polls watched files twice a second. A change repeats only the
steps affected by that file rather than rebuilding the entire environment.

## What it watches

- the project's own Go, `.pw.html`, and `.pw.sql` sources;
- the migration directory;
- the Tailwind input, when Tailwind is enabled;
- anything matched by `dev.watch.includes` in `popcornwave.toml`.

The walk covers the module, not the `[generate]` purposes: any Go source is a
rebuild input, including files no purpose generates from. `.git`, `vendor`,
`node_modules`, `.devbox`, and the `public` tree are always skipped.

`dev.watch.includes` takes relative glob patterns for inputs the walk does not
reach. `dev.watch.excludes` skips a subtree, which is what to reach for when a
large dependency tree makes the walk the slowest step of the loop. Absolute
paths are rejected in both:

```toml
[dev.watch]
includes = ["config.dev.toml", "assets/**/*.svg"]
excludes = ["web/node_modules"]
```

## Services

The services declared in `devbox.json` — Valkey by default — run under the
Devbox process manager with its full-screen terminal UI disabled. Their logs
join the same stream as generation, migration, and application output, one
prefixed line per event, instead of painting over it:

```
[valkey	] 1:M 27 Jul 2026 23:02:32.103 * Ready to accept connections tcp
```

A project that needs no service can drop the package from `devbox.json`;
`pw dev` starts whatever Devbox declares and nothing else.

## Tailwind

During development the watcher always produces **unminified** CSS, regardless of
`assets.tailwind.minify`, because minification is the slow part of the loop. A
failed CSS watcher does not take down the server. `pw dev` keeps running and
falls back to watching the input file directly.

`tailwindcss` must be on `PATH`, which is what `devbox shell` is for. See
[Styling](/guides/styling/).

## Migrations

Pending migrations are applied before the application starts, and again when a
file in the migration directory changes. Turn this off when you would rather
control it yourself:

```toml
[migration]
auto = false
```

## Development identity provider

`pw dev` can run a local OpenID Provider so an OIDC login works before any real
identity provider exists. It signs you in by letting you pick a user from a
list — no password is checked, which is why it never runs outside development.

```toml
[dev.idp]
enabled = true
# config = "devidp.toml"   # roster file, relative to the project
# port = 0                 # 0 reserves an available loopback port
```

The roster lists the selectable users and the claims each one receives:

```toml
[users.admin]
display_name = "Administrator"
extra_scopes = ["admin"]
[users.admin.claims]
email = "admin@example.com"
role = "admin"

[users.guest]
display_name = "Guest User"
[users.guest.claims]
email = "guest@example.com"
```

No client registration or issuer copying is needed. `pw dev` creates an
ephemeral client for the run and passes the application

- `AUTH_OIDC_ISSUER`
- `AUTH_OIDC_CLIENT_ID`
- `AUTH_OIDC_CLIENT_SECRET`

as environment variables. Because environment values outrank TOML, no provider
credential needs to enter a committed config file. Values you exported yourself
are preserved, while the generated client secret changes per run and is never
printed.

Editing the roster reloads it in place: the issuer and the credentials the
running application already holds stay valid, so no restart is needed.

The provider implements Authorization Code with mandatory S256 PKCE, discovery,
JWKS, RS256 ID Tokens, and UserInfo. Refresh tokens, logout, device and client
credentials grants, and consent screens are deliberately absent. See
[`contrib/devidp`](https://github.com/shibukawa/popcornwave/tree/main/contrib/devidp).
`pw build` refuses to build an application that imports it.

For tests, `testutil.WithIdentityProvider` starts the same provider and
`WithLoginUser` pre-selects the subject, so a login completes without a browser.
See [Testing](/guides/testing/).

## Stopping

`Ctrl-C` cancels the whole loop, stopping the application, Tailwind watcher, and
Devbox services. If the application instead exits with an error, `pw dev`
reports `application exited: …` and stops. It does not keep restarting a
process that cannot stay up.
