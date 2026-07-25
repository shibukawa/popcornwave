---
id: data:request-authentication
type: data
title: Request Authentication
---
Authentication middleware records one immutable, protocol-neutral result for downstream handlers and authorization checks.

```yaml
state:
  - unauthenticated
  - authenticated
authenticated_fields:
  subject: stable identity identifier
  principal: optional application-defined typed value
  method: session, passkey, OIDC, bearer, or application-defined method
  session: optional opaque safe metadata
  authorization_scope: optional tenant, roles, permissions, or application-defined claims
  authenticated_at: optional timestamp
  expires_at: optional timestamp
security:
  exclude:
    - password and password hash
    - bearer, refresh, and ID token body
    - raw session cookie or session secret
    - passkey ceremony secret or private key material
    - provider client secret
rules:
  - verify credentials before constructing authenticated state
  - copy or freeze mutable claims before handler dispatch
  - represent anonymous requests explicitly instead of using a missing principal as authority
```
