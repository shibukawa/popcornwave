---
id: flow:session-lifecycle
type: flow
title: Session Lifecycle
---
Session middleware resolves one token into the registered slots of api:session-registry, applies renewal or revocation, and never exposes the token itself.

```yaml
request:
  - read the token cookie once
  - treat a missing token as a browser with no session yet, not as a failure
  - validate token syntax and hash it before store lookup
  - bind the request and response for a store that keeps its records in the browser
  - load each server-placed slot through api:session-store, and decode each cookie-placed slot from its own cookie
  - read a session.Private slot from the server once the session is authenticated, and from its cookie before that
  - reject absolute, idle, or version expiry
  - add the safe slot views and data:request-authentication to the request capsule
  - derive the masked policy:csrf-protection token when enabled
issuance:
  - a bare read issues nothing
  - the first Set on any slot issues the token and writes the record its placement selects, per api:session-manager
  - an anonymous browser therefore holds a session before any login
  - an anonymous session reaches the server only where a session.ServerOnly slot was written
write:
  - a Set is visible to the rest of the request without a re-read
  - a cookie-placed slot writes Set-Cookie immediately and therefore precedes response commitment
  - a server-placed slot is flushed once per request, replacing its record atomically
  - a write touching both placements fails the request rather than committing one of them
renewal:
  - touch only after renewal_interval
  - never extend beyond absolute expiry
  - align cookie lifetime with authoritative server expiry
  - any request counts as activity, including a live-connection reconnect an unattended page makes on its own; requirement:presence-signal is the wanted replacement
login:
  - popcornweb/plugin/auth writes its own slot and calls Rotate, per policy:session-security
  - rotation preserves every other slot, so state written before the login survives it
  - rotation promotes every session.Private slot into the server backend and expires its anonymous cookie, per decision:slot-declared-placement
  - a failed promotion leaves the anonymous record intact and the login unfinished
logout:
  - plugin/auth calls Destroy, which revokes every record and expires every cookie the session owns
  - every registered slot is destroyed together, whatever tier it carries
  - a private slot is not written back to a cookie; nothing travels from the server to the client on the way out
  - a cookie meant to outlive this, such as the policy:session-downgrade hint, is an api:cookie-jar cookie and not a slot
failure:
  malformed_or_expired: clear the session cookies and continue with no session
  store_unavailable: fail closed with a safe service error instead of silently downgrading authentication
  oversized_cookie_slot: refuse the write naming the slot, because a browser drops an oversized cookie silently; an anonymous session.Private slot is not spilled to the server instead
```
