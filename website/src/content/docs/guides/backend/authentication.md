---
title: Authentication
description: Configure OIDC login and let the framework serve the login, callback, and logout endpoints.
sidebar:
  order: 1
---

OIDC usually adds three routes, session resolution, and a trail of protocol
code. Popcorn Wave owns that machinery: configure a provider, and the framework
mounts login, callback, and logout, resolves each request's session, and gives
handlers an identity. The application registers neither routes nor OIDC
callbacks.

## Turning it on

Two things:

```go
// cmd/myapp/main.go — installing the account resolver imports plugin/auth,
// whose extensions serve the endpoints and resolve the session.
import _ "github.com/shibukawa/popcornwave/plugin/session/rdb" // session.backend = "rdb"

func main() {
	handlers.RegisterAccountResolver()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

The storage import is separate from `plugin/auth` on purpose: the auth plugin
links no backend, so an application carries only the one its `session.backend`
selects. `pw init --auth=oidc` writes both lines.

```toml
# config.dev.toml
[session]
enabled = true
backend = "rdb"          # or "cookie" or "redis"; the token stays opaque in all three

[auth]
enabled = true
mode = "oidc_only"

[auth.oidc]
issuer = "https://issuer.example"
client_id = "..."
client_secret = "..."
redirect_url = "https://app.example/auth/callback"
identity_claim = "sub"   # the verified claim that identifies an account
provider_logout = true   # also sign out of the provider
```

`pw init --auth=oidc` writes both, plus the migrations that create the
framework tables. Startup verifies those tables and names the migration to
apply when one is missing.

## The endpoints

| Path | Method | What it does |
| --- | --- | --- |
| `auth.login_path` (`/auth/login`) | GET | redirects to the provider |
| `auth.callback_path` (`/auth/callback`) | GET | verifies the result and starts the session |
| `auth.logout_path` (`/auth/logout`) | POST | ends the session |

`auth.protection.include` lists the paths that require a session; everything
else stays public. An unauthenticated request is redirected through the login
and returned afterwards, or answered with `401` when
`auth.protection.unauthenticated = "unauthorized"`.

**Logout is POST only.** If a link or browser prefetch could end a session,
sign-out would become a denial-of-service surface. A `GET` therefore receives
`405`, and the control is a form:

```html
<form method="post" action={logoutPath}>
  <button type="submit">Sign out</button>
</form>
```

Cross-origin logout is refused: the session cookie is `SameSite=Lax`, and the
endpoint additionally rejects a mismatched `Origin`.

By default, logout also ends the **provider** session. Removing only the local
cookie leaves the user signed in upstream; the next login can return the same
account immediately, making sign-out appear ineffective. The endpoint therefore
redirects through the provider's RP-initiated logout with `client_id` and a
`post_logout_redirect_uri` pointing back to this origin:

```
POST /auth/logout
  → 303 https://issuer.example/end_session?client_id=…&post_logout_redirect_uri=…
  → 302 back to your post-logout page
```

Set `auth.oidc.provider_logout = false` to keep the logout local — appropriate
when the provider is shared with other applications that should stay signed in.
A provider that advertises no `end_session_endpoint` falls back to the local
logout automatically.

After login the browser lands on `auth.post_login_path`, or on the path it was
originally trying to reach. Only a rooted same-site path is accepted, so a login
link cannot be turned into an open redirect.

## Reading the user

The framework resolves the session before your handler runs:

```go
func home(w http.ResponseWriter, r *http.Request) {
	user, signedIn := auth.User(r.Context())
	if signedIn {
		// user.AccountID, user.DisplayName, user.Email, user.Issuer, user.Key
	}
	// ...
}
```

A verified identity does not dictate the application's account model. The
framework calls the resolver registered with `auth.SetAccountResolver`; that
resolver looks up an account and may provision one when
`auth.oidc.auto_provision` permits it. The stable link combines the issuer with
the claim named by `auth.oidc.identity_claim` — never the email address.

An expired or unknown session cookie is discarded silently. Anonymous requests
are a normal state, so `auth.User` simply reports `false`. When it reports a
user, it answers *who*, not *whether that user may perform an action*;
authorization remains in the application.

## Modes

| `auth.mode` | Status |
| --- | --- |
| `oidc_only` | implemented |
| `oidc_passkey`, `passkey_only` | not implemented; startup rejects them |

## The session

The cookie carries an opaque token; where the session itself lives is
`session.backend`. The default `rdb` keeps it in the database through
`plugin/session/rdb`, `redis` keeps it in Redis or Valkey with a server-owned
TTL, and `cookie` seals it into a second cookie for a deployment that wants no
session storage — see [Cookies](/guides/backend/cookies/) for how that choice differs.
`session.ttl` is the absolute lifetime and `session.idle_timeout` the
inactivity one. Logging in rotates the token, which revokes whatever the
browser held before, except under the cookie backend, which cannot revoke a
copy the client already took.

The stored payload holds the account summary and no token body, so a provider
access or ID token never sits in the session.

## Development

A real provider should not be required to exercise a login flow during local
development. `pw dev` can run a development provider instead:

```toml
# popcornwave.toml
[dev.idp]
enabled = true
```

It starts the [development identity provider](/productivity/dev-identity-provider/),
registers a client for the run, and injects `AUTH_OIDC_ISSUER`,
`AUTH_OIDC_CLIENT_ID`, and `AUTH_OIDC_CLIENT_SECRET` — so a project scaffolded
this way has no provider values in any committed file. Signing in means picking
a user from a list; no password is checked, which is why it never runs outside
development.

In tests, `testutil.WithIdentityProvider` starts the same provider and
`WithLoginUser` pre-selects the subject, so one request to `auth.login_path`
completes the entire flow. See [Testing](/productivity/testing/#withidentityprovider).

## Deploying

Deployment removes that convenience. `issuer`, `client_id`, and `client_secret`
must be non-empty or the application refuses to start, naming both the missing
keys and their `AUTH_OIDC_*` environment variables. Supply them — and
`SESSION_SECRET` — through the environment rather than a committed file.

`auth.oidc.redirect_url` may stay empty, in which case the callback URL follows
the request origin. Set it explicitly when the browser-facing origin differs
from what the application sees, and register the same value at your provider.

Register the post-logout URL too: providers reject a `post_logout_redirect_uri`
they do not know. It is `auth.post_logout_redirect` on your public origin —
`https://app.example/` for the default `/`.

The development provider is the exception: it accepts any local post-logout URL
without registration, so a `pw dev` logout works before you have configured
anything anywhere.
