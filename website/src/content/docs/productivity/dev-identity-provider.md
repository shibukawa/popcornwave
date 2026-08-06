---
title: Development Identity Provider
description: Sign in through a real OIDC flow before a real provider exists, with a roster of users you pick from.
sidebar:
  order: 3
---

[`pw dev`](/pw/project/dev/) can run a local OpenID Provider so an OIDC login
works before any real identity provider exists. It signs you in by letting you
pick a user from a list — no password is checked, which is why it never runs
outside development.

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
credentials grants, and consent screens are deliberately absent. `pw build`
refuses to build an application that imports it. See
[`contrib/devidp`](https://github.com/shibukawa/popcornwave/tree/main/contrib/devidp).

For tests, `testutil.WithIdentityProvider` starts the same provider and
`WithLoginUser` pre-selects the subject, so a login completes without a browser.
See [Testing](/productivity/testing/).
