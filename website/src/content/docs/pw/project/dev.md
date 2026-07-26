---
title: pw dev
description: The development loop — services, generation, migrations, CSS, and restart on change.
sidebar:
  order: 3
---

```sh
pw dev
```

The everyday command. It takes no arguments.

## What it does on startup

1. starts the Devbox services declared in `devbox.json`;
2. runs [`pw generate`](/pw/project/generate/);
3. applies pending migrations, unless `migration.auto` is `false`;
4. builds the Tailwind stylesheet and starts its watcher, if Tailwind is enabled;
5. starts the development identity provider, if `dev.idp.enabled` is `true`;
6. builds and runs `project.main`.

Then it polls the watched files twice a second and repeats the relevant steps
when something changes.

## What it watches

- the project's own Go, `.pw.html`, and `.pw.sql` sources;
- the migration directory;
- the Tailwind input, when Tailwind is enabled;
- anything matched by `dev.extra_watch` in `popcornwave.toml`.

`dev.extra_watch` takes relative glob patterns. Absolute paths are rejected:

```toml
[dev]
extra_watch = ["config.dev.toml", "assets/**/*.svg"]
```

## Tailwind

The watcher runs **unminified** during development regardless of
`assets.tailwind.minify`, because minification is the slow part of the loop. If
the watcher exits, `pw dev` keeps going and falls back to watching the input
file directly, so a crashed CSS process does not take the server down with it.

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

You do not register a client or copy an issuer URL. `pw dev` registers an
ephemeral client for the run and hands the application

- `AUTH_OIDC_ISSUER`
- `AUTH_OIDC_CLIENT_ID`
- `AUTH_OIDC_CLIENT_SECRET`

as environment variables. Environment values outrank TOML, so nothing about the
provider belongs in a config file you commit. A value you exported yourself is
left alone. The client secret is generated per run and never printed.

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

`Ctrl-C` cancels the run: the application, the Tailwind watcher, and the Devbox
services are stopped. If the application exits on its own with an error,
`pw dev` reports `application exited: …` and stops, rather than looping on a
process that cannot start.
