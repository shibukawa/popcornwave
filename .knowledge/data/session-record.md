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
  csrf_secret: random session-bound secret when CSRF is enabled
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
rules:
  - store codecs are bounded and reject malformed records
  - expiry is authoritative on the server even when a cookie remains
  - version supports invalidation after incompatible schema or policy changes
  - CSRF secret rotates with session creation and authentication-strength rotation
```
