# session

`session` provides opaque login sessions and the typed cookies an application
writes on its own. The browser receives a random 256-bit token; the store key is
its SHA-256 hash, so a leaked backend dump cannot be replayed as a cookie. Every
authoritative lifetime is decided by the server, even when a cookie survives.

The package contains the `Store[T]` contract, the payload `Codec[T]`, and the
`Manager[T]` that owns cookies and record lifetime, plus `Jar[T]` for
application cookies and `CookieStore` for sessions that need no storage.
`plugin/session/rdb` stores records in a `database/sql` database, and
`plugin/session/redis` stores them in Redis or Valkey. A backend implements the
non-generic `RawStore`, and `Typed[T]` adds the payload type back, which is what
lets one registry hold every backend and an application link only the one it
configured.

```go
raw, _ := rdb.NewStore(db, rdb.Options{})
_ = raw.EnsureSchema(ctx)
manager, _ := session.NewManager(session.Typed[Data](raw, session.JSONCodec[Data]{}), session.Options[Data]{
	TTL:         12 * time.Hour,
	IdleTimeout: time.Hour,
	Cookie:      session.CookieOptions{Secure: true, HTTPOnly: true},
	Subject:     func(data Data) string { return data.AccountID },
})
handler = manager.Middleware(nil)(handler)
```

`Manager.Create` issues a session, `Rotate` revokes the previous record before
issuing a replacement, and `Delete` revokes the record and expires the cookie.
Authentication-strength changes must rotate: reusing a token after login leaves
a fixated session valid.

The middleware resolves the cookie once per request and publishes a safe view
plus the request authentication result. Handlers read it with `session.Read[T]`
and never see the token, the key hash, or the backend client.

A missing, malformed, or expired cookie continues as an explicitly
unauthenticated request with the browser cookie cleared. A backend failure is
answered by the supplied `UnavailableHandler` instead of silently downgrading
the request to anonymous.

Renewal touches a record only after `RenewalInterval` and never extends it past
the absolute expiry. `Version` invalidates every record written before an
incompatible payload or policy change.

Cookie values, key hashes, and stored payloads must never be logged.

## Application cookies

`Jar[T]` reads and writes one typed cookie in one of three protections:
`CookiePlain` is readable and writable by the client, `CookieSigned` is readable
but tamper-evident, and `CookieSealed` is encrypted and authenticated. The typed
API is identical in all three, so a cookie is promoted without touching the
handlers that use it.

```go
keys, _ := session.ParseKeyring(os.Getenv("COOKIE_SECRET")) // openssl rand -base64 32
prefs, _ := session.NewJar[Preferences](nil, session.JarOptions{
	Mode:   session.CookieSigned,
	Keys:   keys,
	Cookie: session.CookieOptions{Name: "pw_prefs", Secure: true, HTTPOnly: true},
	MaxAge: 30 * 24 * time.Hour,
})
handler = prefs.Middleware()(handler)
```

Inside a request, `prefs.Read(ctx)` returns the value and `prefs.Value(ctx)`
returns a handle whose `Set` and `Clear` write the cookie immediately. A value
this jar did not write, or one past its stamped expiry, is cleared from the
browser and read as absent.

The cookie name and, for a session record, the token it belongs to are
authenticated, so a value cannot be moved between cookies or replayed against
another user. The reader also fixes the mode: a sealed cookie downgraded to a
plain value is rejected, not decoded. The first keyring secret writes and
retired ones keep reading, which is what makes a rotation invisible.

## Sessions without storage

`CookieStore` is a `RawStore` that seals the record into a second cookie, bound
to the hash of its own token. A `Manager` over `Typed[T]` of it behaves exactly
like one over `plugin/session/rdb` — the same options, the same `Create`,
`Rotate`, and `Delete`, the same `Read[T]` — so a deployment moves to a database
by setting `session.backend` and adding that backend's import. This one is built
into the framework and needs no import.

It cannot revoke. `Delete` expires the client's copy, but a copy taken earlier
stays valid until its sealed expiry, because no server record exists to remove.
Rotating the secret ends every outstanding session at once. A record that
outgrows the browser cookie budget is refused at the write rather than dropped
silently by the browser. Where either limit matters, run a server-side store.
