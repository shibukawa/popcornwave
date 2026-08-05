---
id: api:session-manager
type: api
title: Session Manager API
---
The session manager owns the opaque token that names one browser and applies the lifetime it is handed, without knowing what any slot means.

```yaml
package: github.com/shibukawa/popcornwave/session
boundary: concept:session-storage-boundary
surface:
  - session.NewManager(Registry, Options) constructs a manager over the api:session-registry slots
  - Manager.Middleware(UnavailableHandler) installs flow:session-lifecycle
  - Manager.Rotate(http.ResponseWriter, *http.Request) error issues a new token and preserves every slot
  - Manager.Destroy(http.ResponseWriter, *http.Request) error revokes the record and expires every cookie the session owns
  - handlers read and write state through api:session-registry and never call the manager
callers:
  driver: popcornwave/plugin/auth, which rotates at an authentication-strength change and destroys at logout
  application: only where it performs its own login without the plugin
token:
  browser_value: 256 random bits in base64url
  store_key: SHA-256 hash of the browser value
  issuance: lazy; the first Set on any slot issues it, so a visitor who writes nothing receives no cookie
placement:
  resolved: when a record is created, not when the process starts, per decision:slot-declared-placement
  consequence: Rotate is also the promotion of every session.Private slot, because it creates the replacement record
  records: one session may hold a cookie-placed record and a server-placed record at once
  atomicity: each placement is replaced atomically, and a write spanning both fails the request rather than committing one half
lifetime:
  source: decision:session-lifetime-owned-by-auth supplies ttl, idle_timeout, and renewal_interval through Options
  absent: with no lifetime source the token cookie is a browser-session cookie and no absolute deadline is stamped
  authority: the stamped deadline is the server's, and the cookie attributes never override it
rules:
  - Rotate revokes the previous record before issuing a replacement, and the replacement carries the same slot values
  - Rotate is the fixation defense, so it changes the token and never the state, which is what lets a login keep what the anonymous browser accumulated
  - Rotate re-resolves placement, so a private slot lands in the server backend and its anonymous cookie is expired in the same response
  - a promotion that fails leaves the previous record intact and the login unfinished, rather than a session holding neither copy
  - Destroy ends the whole session; every registered slot goes together, whatever its placement, per flow:session-lifecycle
  - a cookie a deployment wants to survive a Destroy is an api:cookie-jar cookie and not a registered slot
  - handlers never receive the raw token, key hash, backend client, or mutable stored record
  - middleware supplies the validated slot views through data:request-context-capsule
  - the manager derives data:request-authentication from the plugin/auth slot and derives nothing from any other slot
  - Options rejects an insecure SameSite none cookie, an idle timeout beyond the absolute TTL, and a non-rooted cookie path
  - a token that does not match the issued syntax never reaches the store
  - the manager binds the request and response into every store call, which a client-side store reads through api:session-store RequestBinder
  - the manager is unaware of the selected backend, so decision:cookie-session-storage changes no call site
implemented:
  shape: session.NewManager[T] over one generic payload, with Create, Rotate, Delete, and session.Read[T]
  lifetime: read from the data:session-runtime-config ttl, idle_timeout, and renewal_interval keys
  issuance: at login only, so no anonymous session exists
  subject: Options.Subject derives data:request-authentication from the typed payload
  migration: the generic payload becomes the plugin/auth registered slot, per decision:type-keyed-session-storage
deferred:
  - policy:csrf-protection secret generation and rotation
```
