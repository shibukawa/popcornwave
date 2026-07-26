# session

`session` provides opaque, server-side login sessions. The browser receives a
random 256-bit token; the store key is its SHA-256 hash, so a leaked backend
dump cannot be replayed as a cookie. Every authoritative lifetime lives on the
server, even when a cookie survives.

The package contains the `Store[T]` contract, the payload `Codec[T]`, and the
`Manager[T]` that owns cookies and record lifetime. Implementations live
elsewhere: `plugin/session/rdb` stores records in a `database/sql` database.

```go
store, _ := rdb.NewStore[Data](db, session.JSONCodec[Data]{}, rdb.Options{})
_ = store.EnsureSchema(ctx)
manager, _ := session.NewManager(store, session.Options[Data]{
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
