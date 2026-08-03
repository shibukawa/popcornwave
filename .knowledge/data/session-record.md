---
id: data:session-record
type: data
title: Session Record
---
A stored session record binds an opaque token hash to immutable typed application authentication data and bounded lifetime metadata.

```yaml
stored_fields:
  key_hash: hash of random cookie token
  data: typed immutable T
  created_at: timestamp
  authenticated_at: timestamp
  last_seen_at: timestamp
  expires_at: absolute timestamp
  idle_expires_at: optional timestamp
  authentication_method: string
  csrf_secret: random session-bound secret when CSRF is enabled, per requirement:csrf-token-lifecycle; a visitor with no session carries one in a signed cookie instead, per decision:anonymous-csrf-secret-storage, so this field is the authenticated half only
  version: integer
request_view:
  - typed data
  - timestamps and expiry
  - authentication method
  - version
excluded_from_request_view:
  - raw cookie token
  - key hash
  - backend serialization bytes
  - CSRF secret
assurance:
  today: authenticated_at and authentication_method are the whole freshness and strength surface
  planned: data:session-assurance-state adds the ordered strength label and the provider proof time
rules:
  - store codecs are bounded and reject malformed records
  - expiry is authoritative on the server even when a cookie remains
  - version supports invalidation after incompatible schema or policy changes
  - CSRF secret rotates with session creation and authentication-strength rotation
```
