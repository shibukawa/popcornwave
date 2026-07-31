---
title: Cookies
description: Typed cookies in three protections — client-editable, tamper-evident, and encrypted — and the session storage that continues from them.
sidebar:
  order: 4
---

A cookie is the one piece of application state the client holds, so the first
question is not how to write it but how much the client is allowed to do with
it. Popcorn Wave answers that with three modes over one typed API.

| Mode | Client reads | Client writes | Typical use |
| --- | --- | --- | --- |
| `session.CookiePlain` | yes | yes | display preferences the client owns |
| `session.CookieSigned` | yes | no | values the client may see but not choose |
| `session.CookieSealed` | no | no | anything the client must not read |

The API does not change between them. A cookie that starts out plain during
development becomes signed, then sealed, by editing the mode — the handlers that
read and write it stay as they are.

## Declaring a jar

```go
keys, err := session.ParseKeyring(os.Getenv("COOKIE_SECRET"))

type Preferences struct {
	Theme string `json:"theme"`
	Rows  int    `json:"rows"`
}

prefs, err := session.NewJar[Preferences](nil, session.JarOptions{
	Mode:   session.CookieSigned,
	Keys:   keys,
	Cookie: session.CookieOptions{Name: "pw_prefs", Secure: true, HTTPOnly: true},
	MaxAge: 30 * 24 * time.Hour,
})
```

The `nil` codec means JSON. Anything satisfying `session.Codec[T]` replaces it,
which is where a compact binary encoding goes when the payload grows.

Install the middleware once and the cookie is decoded once per request:

```go
handler = prefs.Middleware()(handler)
```

```go
func Settings(w http.ResponseWriter, r *http.Request) {
	current, ok := prefs.Read(r.Context())
	if !ok {
		current = Preferences{Theme: "system", Rows: 20}
	}
	value, _ := prefs.Value(r.Context())
	_ = value.Set(Preferences{Theme: "dark", Rows: current.Rows})
}
```

`Set` writes the `Set-Cookie` header immediately, so it belongs before the
response body like any other header write, and the new value is what the rest of
the request reads. `Clear` expires the cookie. Outside a middleware chain —
in a background job or a test — `Load`, `Save`, and `Clear` take the request or
the writer directly.

## What each mode guarantees

A plain value is base64 of the payload: the browser reads it with one `atob`
call and can replace it with anything. Treat what comes back as request input
and validate it exactly like a query parameter.

A signed value carries an HMAC. The client still reads the payload, so a signed
cookie is the wrong place for a secret, but a changed one is rejected rather
than decoded. A sealed value is AES-256-GCM: the client sees only ciphertext.

Rejection is deliberate in one more direction. The reader fixes the mode, so a
client that strips a sealed cookie down to a plain value it can author does not
get read as plain — it gets rejected. The cookie name and, for a session record,
the token it belongs to are authenticated too, so a value cannot be moved to
another cookie or replayed against another user.

Where a request carries a value the jar did not write, or one whose stamped
lifetime has passed, the middleware clears it from the browser and continues as
though the request carried nothing. Stale client state is not an error the
application has to handle.

## Secrets and rotation

A secret is 32 or more random bytes, carried as base64:

```bash
openssl rand -base64 32
```

`ParseKeyring` takes the writing secret first and any number of retired ones
after it. Only the first writes; the rest are still accepted for reading, which
is what makes a rotation invisible to a browser holding a value written last
week. Drop a retired secret from the list and every value written under it stops
being accepted at once — the same lever that ends every outstanding cookie
after a leak.

## Two limits worth knowing before you design around cookies

A browser drops an oversized cookie without telling anyone, which would look
like a value that never comes back. `Save` refuses instead, with
`session.ErrCookieTooLarge`, once the name and encoded value together pass about
3.8 KB.

And a `MaxAge` is enforced on both sides. The browser is asked to forget the
cookie, and the same deadline is stamped inside the authenticated payload, so a
client that keeps sending an expired value is refused rather than trusted.

## Sessions: the same cookie, then a database

Login state is not a jar. It belongs to the session manager, which keeps the
browser holding an opaque token and the record wherever storage is configured.
What the cookie mode buys there is a starting point that needs no storage at
all:

```toml
[session]
enabled = true
backend = "cookie"

[session.cookie_store]
secret = "${SESSION_COOKIE_SECRET}"
```

The record is sealed under the hash of its own session token and travels in a
second cookie beside it. It needs nothing else: the cookie backend is built into
the framework, which is why a project can start here with no storage at all.

Moving to server storage later is a configuration line and an import:

```toml
[session]
backend = "rdb"     # or "redis"
```

```go
// cmd/myapp/main.go — the backend a binary contains is the one it imports.
import _ "github.com/shibukawa/popcornwave/plugin/session/rdb"
```

```toml
# backend = "redis"
[session.redis]
dsn = "redis://${REDIS_HOST}:6379/0"
key_prefix = "pw:session:"
```

```go
import _ "github.com/shibukawa/popcornwave/plugin/session/redis"
```

The import is what puts a storage client in the binary, so an application links
the backend it configured and no other — a project on cookies or `rdb` never
carries the Redis client. Configure a backend without its import and startup
stops with the missing line quoted, rather than with a session that fails at the
first login.

Nothing else moves. `session.Read[T]` in a handler, `Create`, `Rotate`, and
`Delete` on the manager, and the middleware that resolves the request are the
same code over `session.CookieStore`, over `plugin/session/rdb`, and over
`plugin/session/redis`. Each implements the same non-generic `session.RawStore`,
and the framework adds the payload type back with `session.Typed`, which is what
lets one configuration value choose between them.

Which one to run is a question about revocation, size, and where expiry is
enforced, and it is worth answering before a deployment rather than after.

A cookie-backed session cannot be revoked. Logout expires the client's copy, but
a copy taken beforehand stays valid until its sealed expiry passes, because
there is no server record to delete. Rotating the sealing secret ends every
outstanding session at once, which is the blunt version of the same control.

The two server backends both revoke immediately and differ mainly in who
collects what nobody revoked. The RDB store keeps a row per session and needs a
periodic sweep, which the auth plugin runs for it. The Redis store writes each
record with a TTL taken from its own deadline, so an abandoned session
disappears on its own and no sweep exists to schedule. Redis and Valkey are both
supported, over `GET`, `SET`, `SET XX`, and `DEL` — no scanning, and no key
outside the configured prefix. Startup pings the server and refuses to serve
against one it cannot reach, rather than answering the first login with a
backend failure.

So: a cookie backend fits development, a single-process deployment, and a small
payload. The RDB store fits a deployment that already runs a database. The Redis
store fits session volume or a renewal rate that a relational store should not
absorb. The code that reads the session never notices the difference.
