---
title: Authentication
description: Configure browser login or verify bearer access tokens for an API server.
sidebar:
  order: 1
---

Browser login usually adds routes, session resolution, and a trail of protocol
code. An API server has a different problem: it receives a bearer token on each
request and must verify the issuer, audience, signature, lifetime, and access
policy before a handler runs. Popcorn Web supports both shapes without making
the application register protocol endpoints or repeat verification middleware.

This page is the reference: keys, endpoints, ceremonies, storage. Which mode a
deployment should pick, and what a session is trusted to do once the login is
over, are decided in [Authentication design](/guides/backend/authentication-design/).

## Turning it on

An entry point and a configuration file:

```go
// cmd/myapp/main.go — installing the account resolver imports plugin/auth,
// whose extensions serve the endpoints and resolve the session. The two
// storage imports are the SQLite ones: the sessions, and the single-use
// records a login ceremony consumes.
import (
	_ "github.com/shibukawa/popcornweb/authstate/sqlite"
	_ "github.com/shibukawa/popcornweb/sessionstore/sqlite"
)

func main() {
	handlers.RegisterAccounts()
	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

The storage imports are separate from `plugin/auth` on purpose: the auth plugin
links no backend, so an application carries only what it configured — and a SQL
store is one package per engine, so switching to PostgreSQL means importing
`sessionstore/postgres` and `authstate/postgres` instead. `pw init --auth=oidc
--db=postgres` writes those lines for you.

```toml
# config.dev.toml
[session]
enabled = true
backend = "rdb"          # cookie, redis, dynamo, and firestore are also available

[auth]
enabled = true
backend = "rdb"          # or "dynamo" or "firestore"
mode = "oidc_only"

[auth.oidc]
issuer = "https://issuer.example"
client_id = "..."
client_secret = "..."
redirect_url = "https://app.example/auth/callback"
identity_claim = "sub"   # the verified claim that identifies an account
provider_logout = true   # also sign out of the provider
```

`pw init --auth=oidc` writes both. With the relational backend it also writes
the migrations that create the framework tables. DynamoDB and Firestore create
records without a relational migration; their deployment requirements are
covered in the corresponding storage guides.

### What a login needs before it starts

Four things have to be true, and startup checks each one rather than
discovering it during a sign-in:

- `session.enabled = true`, since the login has nowhere to land otherwise.
  Which backend holds it is a separate decision — see [Session storage](/guides/storage/session-storage/).
- The backend named by `auth.backend` is linked and reachable. `rdb` requires
  `middleware.rdb`; `dynamo` requires `middleware.dynamo`; `firestore` requires
  `middleware.firestore`. This choice is separate from `session.backend`.
- The backend's deployment resources exist. Relational storage needs its
  migrations, DynamoDB needs its tables, and Firestore needs a Datastore-mode
  database plus any required TTL policies and indexes.
- `issuer`, `client_id`, and `client_secret` all non-empty. Outside loopback
  development, `redirect_url` must also be an absolute URL.

The issuer must be `https`. The one exception is a loopback development
provider, which needs `auth.oidc.allow_loopback_http = true` and must never
carry that flag anywhere else.

## Every option

The `[auth]` keys decide what the framework mounts and what it protects:

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | `false` | the endpoints and the guard exist only when true |
| `backend` | `"rdb"` | ceremony, allowlist, credential, and bootstrap storage: `rdb`, `dynamo`, or `firestore` |
| `mode` | `"oidc_only"` | `oidc_only`, `oidc_passkey`, `passkey_only`, or API-oriented `jwt_only`; see [Modes](#modes) |
| `login_path` | `"/auth/login"` | rooted local path that starts the provider flow |
| `callback_path` | `"/auth/callback"` | rooted local path the provider returns to |
| `logout_path` | `"/auth/logout"` | rooted local path; `POST` only |
| `post_login_path` | `"/"` | where a completed login lands when no path was requested |
| `protection.include` | `[]` | path patterns that require authentication |
| `protection.exclude` | `[]` | patterns carved back out of `include` |
| `protection.unauthenticated` | `"redirect"` | `redirect` through the login, or `unauthorized` with `401` |

Each path is validated as rooted and free of `//` before the server starts, so
a typo is a startup error rather than a route nobody can reach.

The `[auth.oidc]` keys describe the relying party and decide who is admitted:

| Key | Default | Meaning |
| --- | --- | --- |
| `issuer` | *(empty)* | **required**; `https` unless `allow_loopback_http` |
| `client_id` | *(empty)* | **required** |
| `client_secret` | *(empty)* | **required**; masked in the startup summary |
| `redirect_url` | *(empty)* | absolute deployed callback; in loopback development, empty derives `callback_path` from the request origin, and a rooted path derives that path from it |
| `scopes` | `[]` | extra scopes beside `openid` |
| `identity_claim` | `"sub"` | the verified claim that identifies a local account |
| `admission` | `"authenticated"` | `authenticated`, `claim`, `registered`, or `existing` |
| `auto_provision` | `true` | let an unknown verified identity create an account |
| `claim.path` | *(empty)* | JSON Pointer into the verified claims; required by `admission = "claim"` |
| `claim.values` | `[]` | accepted values at that pointer |
| `claim.match` | `"any"` | `any` or `all` |
| `registered_claims` | *(empty)* | claims compared against the allowlist; defaults to `identity_claim` |
| `provider_logout` | `true` | also end the provider session on logout |
| `allow_loopback_http` | `false` | permit an `http` loopback issuer during development |

Supply the three secrets through `AUTH_OIDC_ISSUER`, `AUTH_OIDC_CLIENT_ID`, and
`AUTH_OIDC_CLIENT_SECRET`, or through `${NAME}` references in the file. Both
routes reach the same value; neither belongs in a commit.

## Who gets in

A verified identity is not yet an authorized one, and `admission` is where the
two separate:

| `admission` | Who is admitted |
| --- | --- |
| `authenticated` | everyone the issuer verifies |
| `claim` | identities whose `claim.path` value matches `claim.values` |
| `registered` | identities a deployment listed in the allowlist table beforehand |
| `existing` | only identities the account resolver already knows; requires `auto_provision = false` |

`claim` is the rule for a directory that already carries the answer — a group,
a role, a department. `claim.match = "all"` demands every listed value, which
suits an intersection of groups; `any` admits on the first match.

`registered` is the rule for a closed deployment whose users are known before
their first login. The allowlist table takes an issuer, a claim name, and the
value expected at it, which is why `registered_claims` exists: a deployment
that pre-registers people by employee number compares that claim rather than
the subject it cannot know yet.

Whichever rule admits, the account link is the issuer plus the claim named by
`identity_claim` — never the email address, which providers reassign. Change
`identity_claim` only to something the directory guarantees is stable and
unique for the life of an account.

## JWT-only API servers

`auth.mode = "jwt_only"` is the resource-server mode. It accepts an access
token from `Authorization: Bearer …`, verifies it on every request, and records
the verified caller in the request context. It mounts no login, callback, or
logout endpoint, creates no session, and writes no cookie. A browser flow and a
bearer API are different trust models, so this mode does not combine them.

JWT-only is deliberately absent from the `pw init --auth` choices, which
describe browser login. Use the API-server preset to scaffold it:

```sh
pw init myapi --preset=api-server
```

For a project you already have, link `plugin/auth` through its account resolver
and add the configuration by hand. The smallest stateless deployment is:

```toml
[auth]
enabled = true
mode = "jwt_only"
protection.include = ["/api/**"]
protection.unauthenticated = "unauthorized"

[auth.jwt]
issuer = "https://issuer.example"
audience = ["orders-api"]
algorithms = ["RS256"]
admission = "authenticated"
identity_claim = "sub"
max_token_lifetime = "1h"
revocation.mode = "off"
```

There is no permissive default for the issuer, audience, algorithm allowlist,
admission rule, maximum token lifetime, or revocation mode. Startup refuses a
missing value. Discovery defaults to the issuer's OpenID Connect metadata;
`discovery = "oauth"` uses authorization-server metadata, while `manual`
requires a same-origin `jwks_uri`.

Before admitting the request, the verifier checks the signature against the
configured algorithm allowlist and discovered key set, then checks `iss`,
`aud`, `exp`, `iat`, `sub`, token type, lifetime, and required scopes. The
default required token type is `at+jwt`, which prevents an ID Token signed by
the same issuer from being replayed as an access token. Every rejection has the
same `401` problem response and a `WWW-Authenticate: Bearer` header.

Handlers can read the typed principal or the protocol-neutral authentication
result:

```go
caller, ok := auth.Bearer(r.Context())
if !ok {
	return // the route was not protected, so an anonymous request may reach it
}

accountID := caller.AccountID
claims := caller.Identity.Claims
authentication := pw.RequestAuthentication(r.Context())
```

`admission = "authenticated"` needs no server-side store. `claim` applies a
verified-claim rule and also stays stateless. `registered` reads the relational
allowlist, and any revocation mode other than `off` reads the relational
revocation table; those choices require `middleware.rdb` and the framework
migration. Ending a token before it expires — and undoing that during an
incident — is [Revoking a bearer token](#revoking-a-bearer-token) below.
JWT-only never needs session storage. CSRF protection must be off
for this mode because authority arrives in an explicit header that a browser
does not attach automatically, and there is no session secret to validate.

## Browser-login endpoints

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

## Reading a browser user

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

The three browser modes differ in what establishes an account before one
exists. A passkey alone cannot do that because there is nothing to attach the
first credential to. JWT-only sits outside that lifecycle: the authorization
server has already issued a credential to an API caller.

| `auth.mode` | Account comes from | Everyday login |
| --- | --- | --- |
| `oidc_only` | the provider | the provider |
| `oidc_passkey` | the provider | a passkey, with the provider as recovery |
| `passkey_only` | a login ID and one-time secret an administrator issues | a passkey |
| `jwt_only` | the authorization server that minted the access token | a bearer token on every API request |

A mode reads only its own settings and refuses one it cannot honor, so a
leftover `AUTH_OIDC_ISSUER` under `passkey_only` fails at startup rather than
suggesting a provider is in the loop. A setting that is silently ignored reads
as configured security.

### Passkey endpoints

The modes that mount ceremonies serve five paths under `auth.passkey.path`,
default `/auth/passkey`. They are `POST` and JSON only, and the bootstrap
endpoint exists under `passkey_only` alone:

```
POST /auth/passkey/login/begin      POST /auth/passkey/login/finish
POST /auth/passkey/register/begin   POST /auth/passkey/register/finish
POST /auth/passkey/bootstrap        (passkey_only)
```

The framework serves them but cannot call `navigator.credentials` for the page,
so a project carries a small script that converts between the Base64url the
endpoints speak and the ArrayBuffers the WebAuthn API wants. `pw init` writes
one into `public/passkey.js`.

A passkey mode adds a second account seam. `auth.SetAccountResolver` answers
*which account is this verified identity*, and `auth.SetAccountLookup` answers
*which account is this identifier* — the direction a passkey assertion needs,
because the credential names the account directly and there is no external
identity to link.

### Reach a passkey deployment by name

A WebAuthn relying party is scoped to a **domain**, and an IP literal can never
be one. Use `http://localhost:8080`, not `http://127.0.0.1:8080`. WebAuthn also
treats `localhost` as a secure origin, so local development needs no
certificate and no tunnel.

### The first sign-in under `passkey_only`

An administrator issues a login ID and a one-time secret with
`auth.IssueBootstrapCredential`; the raw secret is returned once and only its
digest is stored. Redeeming it grants a ticket that authorizes exactly one
registration — **not** a session. The request stays unauthenticated until the
passkey is persisted, so no handler can mistake a redeemed secret for a login.

`auth.bootstrap.issue_ttl` bounds delivery and `auth.bootstrap.enrollment_ttl`
bounds the ceremony that follows, which is why the two defaults differ by
hours.

## The browser session

The cookie carries an opaque token; where the session itself lives is
`session.backend`, and that choice is independent of everything above.
[Session storage](/guides/storage/session-storage/) covers the five backends, their required
keys, and what each one gives up. The lifetime is declared here rather than
there: `auth.session.ttl` bounds the absolute one and
`auth.session.idle_timeout` the inactivity one, because an expiry states how
long a proof of identity stays good. Logging in rotates the token,
which revokes whatever the browser held before — except under the cookie
backend, which cannot revoke a copy the client already took.

The stored payload holds the account summary and no token body, so a provider
access or ID token never sits in the session.

## Ending a browser session early

Every credential this page issues has an expiry, and the expiry is never the
interesting case. What matters during an incident is ending one *before* it —
and the two kinds of credential end differently, because one of them is a record
this application owns and the other is a token somebody else minted. This
section is the session; the next is the token.

A browser session ends early in three ways, and only the first is a request the
person makes themselves.

**Logout.** `POST` to `auth.logout_path` destroys the session record and expires
every cookie the session owns. [Sessions](/guides/backend/sessions/#logging-out-ends-everything)
covers what survives it and why.

**A new login.** Signing in rotates the token, so whatever the browser held
before stops being accepted. That is what makes a shared terminal safe to hand
over, and it is also why a session fixation attempt gains nothing.

**Suspending or deleting the account.** This is the one an operator reaches for,
and it works on sessions nobody is going to log out of. An authenticated request
re-reads the account behind its session through the `AccountLookup` the
application installed, and a session whose account is suspended or gone is
destroyed there rather than at its own expiry. The re-read is bounded: at most
once every 30 seconds per account, because doing it on every request would put a
database round trip in front of every authenticated page.

That interval is the honest answer to "how fast does a suspension take effect".
Call `auth.ForgetAccount(accountID)` from whatever performs the suspension and
the next request reads the account again immediately — but the call is
process-local, so a deployment running several instances still waits out the
interval on the others. Promise the interval, not the call.

An account store that cannot answer at all refuses the request with `503`. The
credential was not judged, retrying may succeed, and admitting during an outage
would make every suspension conditional on that store being up.

One backend cannot participate in the first of these. Under
`session.backend = "cookie"` the session record is in the browser, so logout can
only expire the copy it can reach; a copy taken earlier keeps working until the
seal expires. Suspension still lands, because that check happens on the request
rather than on the record — but if ending sessions on demand matters, the record
belongs somewhere the server can delete it. See
[Session storage](/guides/storage/session-storage/#cookie--no-storage-at-all).

## Revoking a bearer token

A verified access token is valid until it expires, and that is the one property
a resource server cannot change on its own: this application did not mint the
token and cannot withdraw it at the issuer. So when a token leaks, or an account
turns out to be compromised, every copy of that credential keeps working until
`exp` — unless the application keeps its own list of tokens it will no longer
honor. Revocation is that list, and it applies to
[`jwt_only`](#jwt-only-api-servers) alone.

:::note[What it requires]
The list is a relational table, so revocation needs `middleware.rdb`,
`auth.backend = "rdb"`, and the framework migration that creates
`popcornweb_auth_revocation`. A deployment on `admission = "authenticated"`
with `revocation.mode = "off"` needs no database at all; turning revocation on
is what creates the requirement.
:::

### Turning it on

`revocation.mode` has no default. Startup refuses a missing value, because
running without a revocation path should be a decision on the page, not an
omission:

```toml
[auth.jwt]
issuer = "https://issuer.example"
audience = ["orders-api"]
algorithms = ["RS256"]
admission = "authenticated"
identity_claim = "sub"
max_token_lifetime = "1h"
revocation.mode = "both"      # off, token, subject, or both
```

`off` is a legitimate answer, not a stub: a deployment whose tokens live for
five minutes may decide the exposure window is acceptable and skip the database
requirement entirely. Do not use revocation as a substitute for short lifetimes
— the list bounds the damage of a leak, it does not shrink the window in which
every ordinary token is trusted.

With any mode other than `off`, every admitted request is checked against the
list after signature and claim verification pass. A revoked token is refused
with the same `401` as an expired one — a caller probing the API cannot tell
which refusal it got, and that is intentional.

### The two forms

`token` revokes one credential; `subject` revokes an identity. They answer
different incidents, and `both` is the ordinary choice because neither
substitutes for the other.

Revoking a **token** is the narrow act, for a leaked credential whose owner is
otherwise fine. The entry is keyed on the token's `jti` claim, which is why
selecting this form makes `jti` mandatory — a token nobody can name is a token
nobody can revoke.

Revoking a **subject** is the broad act, for a compromised account: it refuses
every token issued to that identity *before now*. Enumerating the outstanding
`jti` values of a stolen account is exactly what nobody can do, so the entry
stores a timestamp instead, and the check compares it against each token's
`iat`. The account is not banned — once the person re-authenticates at the
issuer, the fresh token postdates the stamp and is admitted. Banning an account
is an [admission](#who-gets-in) decision, not a revocation.

Both calls name the issuer explicitly. The entry is scoped by it, and a call
that inferred the issuer from configuration would silently write to the wrong
scope if the deployment ever gained a second one.

### Revoking from your own code

There is no HTTP endpoint for any of this. An endpoint that revokes tokens needs
an authorization rule the framework has no basis to write, so the surface is a
set of Go calls, and your application decides who may reach them:

```go
// internal/admin/revoke.go
package admin

import (
	"net/http"

	"github.com/shibukawa/popcornweb/plugin/auth"
	"github.com/shibukawa/popcornweb/pw"
)

const issuer = "https://issuer.example" // the auth.jwt.issuer this server verifies

// RevokeAccount withdraws every outstanding token of one identity.
// Route it however your operators reach admin actions; the framework
// deliberately mounts nothing here.
func RevokeAccount(w http.ResponseWriter, r *http.Request) {
	identityKey := r.PathValue("identity")
	err := auth.RevokeSubject(r.Context(), issuer, identityKey,
		"compromised account, support ticket 4211")
	if err != nil {
		pw.WriteProblem(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

`auth.RevokeToken(ctx, issuer, tokenID, note)` is the single-token counterpart.
The note is for your future self reading the table during an incident; the
framework never shows it to a caller.

Revoking an identifier that was never presented **succeeds**. The caller cannot
know whether a stolen token was ever used here, and an error would make the safe
reflex — revoke first, investigate second — needlessly noisy.

A revocation entry needs no expiry from you. It lives for `max_token_lifetime`
past its stamp, which by construction outlives every token it must refuse, and
the store sweeps it afterwards.

### Undoing a mistake

`auth.ReinstateToken` and `auth.ReinstateSubject` delete an entry. This is
removal, not an undo with history: every unexpired token the entry was refusing
works again at the next request. For an administrative view that must not guess,
`auth.TokenRevoked` and `auth.SubjectRevoked` report the stored stamp and
presence, reading the store directly rather than any per-request cache.

### When the store cannot answer

`revocation.on_unavailable` defaults to `refuse`, and the refusal is a `503`,
not a `401` — the credential was not judged, and retrying may succeed. Admitting
on an outage would turn the revocation table into an optimization and make every
revocation conditional on infrastructure being up. It is the same reasoning the
account store follows one section above, for the same reason.

The `admit` override exists as an incident lever, not a deployment posture: with
it set, every revoked token works again for the duration of the outage, and the
configuration advisories report it at error level so it does not quietly outlive
the incident.

By default every admitted request reads the store.
`revocation.max_propagation_delay` permits a small per-process cache instead,
and its value is the honest answer to "how fast does a revocation take effect" —
a token revoked now may be admitted for up to that long. Leave it at zero unless
the extra read shows up in your latency budget. The three keys are listed with
the rest of `[auth.jwt]` in the [configuration
reference](/reference/configuration/#authjwt).

## Development

A real provider should not be required to exercise a login flow during local
development. `pw dev` can run a development provider instead:

```toml
# popcornweb.toml
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
must be non-empty. `redirect_url` must be the absolute deployed callback;
`pw doctor` reports an empty or path-only value as an error outside `dev`.
Supply the provider values through `AUTH_OIDC_ISSUER`,
`AUTH_OIDC_CLIENT_ID`, and `AUTH_OIDC_CLIENT_SECRET`, or through `${NAME}`
references, rather than committing them. A cookie-backed session adds one more
secret of its own — see [Session storage](/guides/storage/session-storage/#cookie--no-storage-at-all).

`redirect_url` has to be the URL your provider has registered, character for
character. It is the value the framework sends as `redirect_uri`, so a callback
that differs from the registration is refused by the provider before the
application ever sees it.

There is one development-only exception. With `allow_loopback_http = true`, an
empty `redirect_url` becomes the current request's scheme and `Host` plus
`callback_path`; a rooted path uses that path instead. The request `Host` must
be `localhost`, a `*.localhost` name, or a loopback IP such as `127.0.0.1` or
`::1`. This lets a login begun on `localhost:8080` return there, while one begun
on `127.0.0.1:8080` returns to that distinct browser origin. Do not use this
rule behind a public host or as a substitute for registering a production URL.

Register the post-logout URL too: providers reject a `post_logout_redirect_uri`
they do not know. The framework sends the root of the request origin, so
register `https://app.example/` for an application served there.

The development provider is the exception: it accepts any local post-logout URL
without registration, so a `pw dev` logout works before you have configured
anything anywhere.
