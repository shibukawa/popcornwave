---
id: policy:session-security
type: policy
title: Session Security Policy
---
Login sessions use opaque server-side state with bounded lifetime, fixation resistance, and secret-safe diagnostics.

```yaml
token:
  entropy: at least 256 random bits
  browser_value: canonical opaque encoding
  store_key: cryptographic hash of browser value
cookie_defaults:
  http_only: true
  secure: true outside explicit loopback development
  same_site: lax
  path: /
rules:
  - rotate after login, privilege change, and other authentication-strength changes
  - revoke server state on logout
  - enforce absolute expiry and optional idle expiry
  - protect state-changing cookie-authorized requests with policy:csrf-protection
  - never log cookie values, token hashes, stored authentication data, or backend credentials
  - reject insecure SameSite none cookies
  - bind authorization to validated data:request-authentication, never cookie presence alone
  - sqlite://:memory: is process-local and unsuitable for multi-process production sessions
```
