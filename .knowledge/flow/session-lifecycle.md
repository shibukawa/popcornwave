---
id: flow:session-lifecycle
type: flow
title: Session Lifecycle
---
Session middleware resolves one cookie into a validated request session and applies renewal or revocation without exposing its token.

```yaml
request:
  - read the configured cookie once
  - treat a missing cookie as an explicit unauthenticated request
  - validate token syntax and hash it before store lookup
  - load through api:session-store
  - reject absolute, idle, or version expiry
  - add the safe data:session-record view and data:request-authentication to the request capsule
  - derive the masked policy:csrf-protection token when enabled
renewal:
  - touch only after renewal_interval
  - never extend beyond absolute expiry
  - align cookie lifetime with authoritative server expiry
login:
  - api:session-manager creates or rotates after successful authentication
logout:
  - api:session-manager deletes the store record and expires the cookie
failure:
  malformed_or_expired: clear the cookie and continue unauthenticated
  store_unavailable: fail closed with a safe service error instead of silently downgrading authentication
```
