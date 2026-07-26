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
// cmd/myapp/main.go — installing the account resolver imports plugin/auth,
// whose extensions serve the endpoints and resolve the session.
func main() {
	handlers.RegisterAccountResolver()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

```toml
# config.dev.toml
[session]
enabled = true
backend = "rdb"          # sessions are opaque and stored server-side

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
`client_id` and a `post_logout_redirect_uri` pointing back at this origin:

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

The account behind a verified identity is yours to decide: the framework calls
the resolver you registered with `auth.SetAccountResolver`, which looks the
identity up and may provision one when `auth.oidc.auto_provision` permits it.
The link is the issuer plus the claim `auth.oidc.identity_claim` names — never
the email address.

An expired or unknown session cookie is dropped silently; an anonymous request
is a normal state, so `auth.User` reports `false`. It tells you *who*, never
*whether they may*: authorization stays yours.

## Modes

| `auth.mode` | Status |
| --- | --- |
| `oidc_only` | implemented |
| `oidc_passkey`, `passkey_only` | not implemented; startup rejects them |

## The session

The cookie carries an opaque token; the session itself lives in the database
through `plugin/session/rdb`, so it can be expired and revoked server-side.
`session.ttl` is the absolute lifetime and `session.idle_timeout` the
inactivity one. Logging in rotates the token, which revokes whatever the
browser held before.

The stored payload holds the account summary and no token body, so a provider
access or ID token never sits in the session.

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
