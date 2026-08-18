---
title: Sessions
description: Declaring per-browser state by type, stating what the client may do with it, and following it from an anonymous visit into a signed-in one.
sidebar:
  order: 2
---

A locale preference and a credential are both session state, and they want
opposite things. The front end should read one and never see the other. One can
survive a client edit and be treated as input; the other has to stop being valid
the moment an account is disabled.

Popcorn Web asks you to say which is which on the line that declares the type.
The deployment is then left with one choice, and it is the one an operator is
actually qualified to make: which server backend the server-placed values use.

## Declaring a piece of state

```go
// cmd/myapp/main.go — after every package init has run, like pw.RegisterConfig.
pw.RegisterSessionStore[Density]("density", session.Shared)
pw.RegisterSessionStore[Locale]("locale", session.ReadOnly)
pw.RegisterSessionStore[Cart]("cart", session.Private)
pw.RegisterSessionStore[Grants]("grants", session.ServerOnly)
pw.RegisterSessionStore[TokenScopes]("scopes", session.RequestScope)
```

The Go type is the key. A package reads its own state without importing the
package that owns the layout, a misspelled name is a compile error rather than
an empty value, and two packages that want to share a slot have to share the
type — which makes the sharing visible in the import graph. Registering the
same type twice, or the same name twice, panics at startup rather than quietly
replacing what was there.

The second argument is the whole of what a developer decides:

| Placement | The client can | Where it lives |
| --- | --- | --- |
| `session.Shared` | read and write | a plain cookie, necessarily |
| `session.ReadOnly` | read, not change | a signed cookie, necessarily |
| `session.Private` | neither | a sealed cookie while anonymous, the configured backend afterwards |
| `session.ServerOnly` | neither | the configured backend, always |
| `session.RequestScope` | nothing; it never travels | process memory, for one request |

Five rows, five questions, asked in this order:

- Is it rebuilt from an authoritative source on every request?
  `RequestScope`. The scope set a bearer token resolves to against the
  authentication database is the standing example: it is read fresh at the top
  of each request precisely so a revocation there is seen on the next one, and
  it is never persisted anywhere.
- Should the front end be able to change it? `Shared`. A display-density
  toggle, a dismissed notice, the last-used tab. A decoded value is request
  input, and you validate it like a query parameter.
- Should the front end be able to read it? `ReadOnly`. The locale or tenant
  label the server chose and the client may display. The payload stays legible,
  so it carries no secret.
- Must it be revocable before anyone signs in, or can it grow past a cookie?
  `ServerOnly`. A stored secret the client must never hold even sealed — the
  refresh token taken at login — or a draft that grows without bound.
- Otherwise `Private`, which is the default in every sense worth having. The
  cart an anonymous visitor starts and a signed-in user keeps is its shape.

One thing none of the five rows covers: a preference that should follow the
account across browsers. A session names one browser and dies at logout, so
state that belongs to the user rather than the browser lives in the
application's own database, keyed by the account.

A value the client writes cannot live on the server, so the first two stored
rows are cookies by definition rather than by configuration. Only `Private` and
`ServerOnly` leave the deployment anything to decide, and `RequestScope` leaves
nothing to anyone: no cookie, no record, no keyring.

## How long it lives

Placement is one question. How long the value lasts is a second, independent
one, stated on the same line:

```go
pw.RegisterSessionStore[Draft]("draft",     session.Shared,   session.ExpiresAfter(time.Hour))
pw.RegisterSessionStore[Density]("density", session.Shared,   session.OutlivesSession(90*24*time.Hour))
pw.RegisterSessionStore[Locale]("locale",   session.ReadOnly, session.OutlivesSession(session.BrowserMax))
pw.RegisterSessionStore[Cart]("cart",       session.Private)
```

Say nothing and the slot is tied to the session: it dies at sign-out, and its
cookie tracks the session rather than the browser window.

`ExpiresAfter` ends a value early, whatever its placement. A record-placed slot
carries its own deadline inside the record and reads as absent once it passes,
while the session and its other slots continue — which is what a rotating
secret or a short admission window wants.

`OutlivesSession` exempts a value from sign-out, and only the two cookie-placed
tiers may ask for it. A record cannot outlive its own destruction, so the
refusal is a registration error rather than a surprise at logout.

`BrowserMax` is the honest name for "indefinitely." HTTP has no such state: a
cookie with no `Max-Age` dies when the browser closes, and one with a `Max-Age`
is capped — currently at 400 days.

One thing the same argument does not buy equally. A `ReadOnly` value is signed,
so its expiry is inside the authenticated payload and a stale value presented
later is refused. A `Shared` value is plain and carries no stamp at all, so its
duration is the `Max-Age` attribute alone — a promise to a client that can
rewrite the value anyway.

A `RequestScope` slot refuses all three options at registration. Its lifetime
is the request, fixed; stating another one is a contradiction, and usually
means a placement was edited without its options.

## State that is never stored

`RequestScope` is the placement for a value whose freshness matters more than
what it costs to rebuild. Nothing about it reaches the browser or a store: a
`Set` is visible to the rest of the same request, issues no token, and writes
no cookie, and the next request starts empty.

The shape it serves is a middleware deriving a fact from its source of record
and handlers below reading it:

```go
// The auth middleware resolves the bearer token against the database once,
// at the top of the request. Every handler below reads the result.
scopes, err := lookupScopes(r.Context(), token)
if err == nil {
	if handle, ok := session.Value[TokenScopes](r.Context()); ok {
		handle.Set(TokenScopes{Scopes: scopes})
	}
}
```

Because the value is rebuilt from the database on every request, revoking a
scope there is seen on the very next request. There is no cached copy to
invalidate, because there is no copy.

That is also the boundary. If rebuilding is expensive and a bounded staleness
is acceptable, `RequestScope` is the wrong tool — declare the slot
`session.Private` with `session.ExpiresAfter` and you have a cache with a
stated shelf life. Reach for `RequestScope` only when a stale read is the thing
you cannot afford.

## What the browser receives, and when

Nothing, until something is written.

```go
func home(w http.ResponseWriter, r *http.Request) {
	cart, ok := session.Load[Cart](r.Context())   // issues nothing
	if !ok {
		cart = Cart{}
	}
	...
}
```

A bare read costs the browser no cookie and the server no row. The token is
issued by the first `Set`, which means a crawler that walks the whole site
without adding anything to a cart leaves no trace to sweep up later.

```go
handle, ok := session.Value[Cart](r.Context())
if !ok {
	// The type is not registered, or the middleware did not run.
	return
}
if err := handle.Set(cart); err != nil {   // issues the token here
	return
}
```

That token is 256 random bits and nothing else. The store is keyed by its
SHA-256 hash, so a leaked backend dump cannot be replayed as a cookie. Handlers
never see the token, the hash, the placement, or the backend client.

## The move that happens at login

While a visitor is anonymous, a private value rides a sealed cookie — no server
row, whatever backend the deployment configured. At the login rotation it moves
to that backend, and the anonymous cookie is expired in the same response. That
is what makes `Private` the placement to reach for by default.

Nothing new implements this. Logging in already rotates the token to close
session fixation, and rotation was already "revoke the old record, create a
replacement carrying the same values." Resolving the placement when the record
is created rather than when the process starts makes promotion fall out of the
rotation that was happening anyway. There is no separate hook, no second write
path, and no window in which a session is half-promoted.

The consequence a user notices is that the cart survives the sign-in. The
consequence an operator notices is that bots and one-off visits never reach the
session table at all.

It is not free, and the price lands in one place. An anonymous private value is
bounded by the browser cookie budget, about 3.8 KB for the name and encoded
value together. Go past it and the write is refused —
`session.ErrCookieTooLarge`, naming the slot — rather than spilling to the
server behind your back. Anything that can grow without bound while anonymous
is declared `ServerOnly` and pays for its row from the first write.

## Logging out ends everything

`auth` logout destroys the session: every record is revoked and every cookie the
session owns is expired — every slot except the ones that declared
`OutlivesSession`. The anonymous cart goes with it. The display language does
not. A `RequestScope` value is untouched within its request, because the
session stored nothing of it to take back — and it is gone at the next request
regardless.

A cookie that belongs to no session at all can still use `session.Jar`
directly. That exceptional path is covered [at the end of this page](#using-cookies-directly).
The sign-in hint is one example: it describes a session that has already ended
and therefore cannot share that session's lifetime.

## When things go wrong

A missing token is a browser with no session yet, not a failure. A malformed or
expired one is cleared and the request continues with no session.

A backend that cannot be reached is different: the middleware answers `503`
rather than quietly downgrading the request to anonymous. "The database is down"
and "you are not signed in" must not look the same to an application deciding
what to show.

The framework installs the session middleware, and `plugin/auth` drives it. A
project that imports no authentication still declares slots and still reads them
back; what it does not get is a lifetime, because nothing declares one. The
token cookie then lasts as long as the browser keeps it, and no absolute
deadline is stamped.

## Where the bytes go

Everything above is what a handler sees. Where a server-placed slot actually
lands, what bounds it, what each backend costs, and the one keyring that signs
and seals it are all one deployment decision, and they live in
[session storage](/guides/storage/session-storage/).

## Using cookies directly

For new application state, prefer `pw.RegisterSessionStore`. It gives the value
a typed home, applies the session lifetime and logout rules, and lets the
placement state whether the browser may read or change it. Creating a cookie jar
directly does not add a more capable version of that model; it opts out of it.

Use `session.Jar` directly only when the cookie must be independent of the
session lifecycle. The usual cases are keeping an existing cookie format while
older code or clients are still in use, or interoperating with a protocol that
already defines its own cookie. The sign-in hint used by `plugin/auth` is a
framework example.

```go
type LegacyPreference struct {
	Density string `json:"density"`
}

preferences, err := session.NewJar[LegacyPreference](nil, session.JarOptions{
	Mode:   session.CookiePlain,
	Cookie: session.CookieOptions{Name: "legacy_density", Secure: true, HTTPOnly: true},
	MaxAge: 30 * 24 * time.Hour,
})
if err != nil {
	return err
}

handler = preferences.Middleware()(handler)
```

The middleware makes `Read`, `Value().Set`, and `Clear` available through the
request context. Choose `CookiePlain`, `CookieSigned`, or `CookieSealed` to match
the contract of the cookie you must preserve. A plain value is client input and
must be validated. Writes still have to happen before the response body, and a
cookie is still limited to roughly 3.8 KB.

If no existing contract requires a standalone cookie, declare the value as
`session.Shared`, `session.ReadOnly`, or `session.Private` instead. Those
placements cover the same client-access decisions while keeping the value under
the session's lifecycle.
