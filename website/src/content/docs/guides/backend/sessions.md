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

Popcorn Wave asks you to say which is which on the line that declares the type.
The deployment is then left with one choice, and it is the one an operator is
actually qualified to make: which server backend the server-placed values use.

## Declaring a piece of state

```go
// cmd/myapp/main.go — after every package init has run, like pw.RegisterConfig.
pw.RegisterSessionStore[Density]("density", session.Shared)
pw.RegisterSessionStore[Locale]("locale", session.ReadOnly)
pw.RegisterSessionStore[Cart]("cart", session.Private)
pw.RegisterSessionStore[Grants]("grants", session.ServerOnly)
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

Four rows, four questions, asked in this order:

- Should the front end be able to change it? `Shared`. A decoded value is
  request input, and you validate it like a query parameter.
- Should the front end be able to read it? `ReadOnly`. The payload stays
  legible, so it carries no secret.
- Must it be revocable before anyone signs in, or can it grow past a cookie?
  `ServerOnly`.
- Otherwise `Private`, which is the default in every sense worth having.

A value the client writes cannot live on the server, so the first two rows are
cookies by definition rather than by configuration. Only the last two leave the
deployment anything to decide.

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
not.

A cookie that belongs to no session at all still uses
[`session.Jar`](/guides/backend/cookies/) directly. That is where the sign-in
hint lives, because it needs its own keyring and describes a session that has
already ended.

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
