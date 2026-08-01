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
  - authentication-strength change includes the re-proof of policy:reauthentication, which also resets idle expiry
  - expiry is a bound on the whole session, not a substitute for the per-operation assurance of concept:assurance-axes
  - revoke server state on logout
  - enforce absolute expiry and optional idle expiry
  - protect state-changing cookie-authorized requests with policy:csrf-protection
  - never log cookie values, token hashes, stored authentication data, or backend credentials
  - reject insecure SameSite none cookies
  - bind authorization to validated data:request-authentication, never cookie presence alone
  - sqlite://:memory: is process-local and unsuitable for multi-process production sessions
  - a cookie-backed record is sealed under policy:cookie-value-protection and bound to the hash of its own token
  - a cookie backend cannot revoke an issued record, so a deployment that must end sessions on demand uses a server-side store
```
