---
title: Authentication
description: Configure OIDC login and let the framework serve the login, callback, and logout endpoints.
sidebar:
  order: 10
---

Popcorn Wave serves the authentication endpoints itself. You configure a
provider; the framework mounts login, callback, and logout, resolves the session
on every request, and hands your handlers an identity. There is no route to
register and no OIDC code to write.

## Turning it on

Two things:

```go
// cmd/myapp/main.go
import _ "github.com/shibukawa/popcornwave/auth"
```

```toml
# config.dev.toml
[auth]
enabled = true
mode = "oidc"

[auth.oidc]
issuer = "https://issuer.example"
client_id = "..."
client_secret = "..."
provider_logout = true   # also sign out of the provider

[session]
enabled = true
secret = "..."     # SESSION_SECRET in deployments
```

`pw init --auth=oidc` writes both. The blank import is what registers the
implementation: with `auth.enabled = true` and no provider registered, startup
fails and names the import you are missing.

## The endpoints

| Path | Method | What it does |
| --- | --- | --- |
| `auth.login_path` (`/auth/login`) | GET | redirects to the provider |
| `auth.callback_path` (`/auth/callback`) | GET | verifies the result and starts the session |
| `auth.logout_path` (`/auth/logout`) | POST | ends the session |

**Logout is POST only.** A logout a link or a browser prefetch can trigger is a
denial-of-service surface, not a convenience, so a `GET` gets `405` and the sign
out control is a form:

```html
<form method="post" action={logoutPath}>
  <button type="submit">Sign out</button>
</form>
```

Cross-origin logout is refused: the session cookie is `SameSite=Lax`, and the
endpoint additionally rejects a mismatched `Origin`.

Logout also ends the **provider** session. Dropping only the local cookie leaves
the user signed in at the provider, so the next login returns the same account
without asking and the sign-out looks like it did nothing. The endpoint
therefore redirects through the provider's RP-initiated logout with
`id_token_hint`, `client_id`, and a `post_logout_redirect_uri` pointing back at
`auth.post_logout_redirect`:

```
POST /auth/logout
  → 303 https://issuer.example/end_session?client_id=…&id_token_hint=…&post_logout_redirect_uri=…
  → 302 back to your post-logout page
```

Set `auth.oidc.provider_logout = false` to keep the logout local — appropriate
when the provider is shared with other applications that should stay signed in.
A provider that advertises no `end_session_endpoint` falls back to the local
logout automatically.

After login the browser lands on `auth.post_login_redirect`, after logout on
`auth.post_logout_redirect`. Both must be absolute paths on this origin —
an absolute URL is rejected at startup rather than becoming an open redirect.

## Reading the user

The framework resolves the session before your handler runs:

```go
func home(w http.ResponseWriter, r *http.Request) {
	identity, signedIn := pw.CurrentUser(r.Context())
	if signedIn {
		// identity.Subject, identity.Name, identity.Email
		role, _ := identity.Claim("role")
		_ = role
	}
	// ...
}
```

`Identity` is what the provider proved, not an account record. `Subject` is
stable within `Issuer`; resolve your own account from it. An expired, forged, or
malformed session cookie is dropped silently — an anonymous request is a normal
state, so `CurrentUser` simply reports `false`.

`pw.CurrentUser` tells you *who*, never *whether they may*. Authorization stays
yours.

## Modes

| `auth.mode` | Status |
| --- | --- |
| `oidc` | implemented |
| `oidc_passkey` | OIDC login today; passkey enrollment is not implemented yet |
| `passkey_only` | not implemented; `pw init` scaffolds it with `enabled = false` |

## The session

The session is a signed cookie — `HttpOnly`, `SameSite=Lax`, `Secure` on HTTPS —
carrying the subject, a few claims, and the ID Token used as the logout hint,
valid for `session.ttl`. The ID Token never reaches a handler; if a provider's
claims push the cookie past its size bound, the hint is dropped rather than the
session.

There is no server-side store to run. `session.secret` signs the cookie;
authentication refuses to start without one, and rotating it invalidates every
session.

Keep the identity small: anything bigger belongs in your own storage, keyed by
subject.

## Development

You do not need a real provider to build a login. `pw dev` can run one:

```toml
# popcornwave.toml
[dev.idp]
enabled = true
```

It starts the [development identity provider](/pw/project/dev/#development-identity-provider),
registers a client for the run, and injects `AUTH_OIDC_ISSUER`,
`AUTH_OIDC_CLIENT_ID`, and `AUTH_OIDC_CLIENT_SECRET` — so a project scaffolded
this way has no provider values in any committed file. Signing in means picking
a user from a list; no password is checked, which is why it never runs outside
development.

In tests, `testutil.WithIdentityProvider` starts the same provider and
`WithLoginUser` pre-selects the subject, so one request to `auth.login_path`
completes the entire flow. See [Testing](/guides/testing/#withidentityprovider).

## Deploying

`issuer`, `client_id`, and `client_secret` must be non-empty or the application
refuses to start, naming the missing keys and their `AUTH_OIDC_*` environment
variables. Supply them from the environment rather than a committed file, along
with `SESSION_SECRET`.

`auth.oidc.redirect_url` may stay empty, in which case the callback URL follows
the request origin. Set it explicitly when the browser-facing origin differs
from what the application sees, and register the same value at your provider.

Register the post-logout URL too: providers reject a `post_logout_redirect_uri`
they do not know. It is `auth.post_logout_redirect` on your public origin —
`https://app.example/` for the default `/`.

The development provider is the exception: it accepts any local post-logout URL
without registration, so a `pw dev` logout works before you have configured
anything anywhere.
