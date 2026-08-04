---
id: policy:session-security
type: policy
title: Session Security Policy
---
The session token is unguessable, rotated at every authentication-strength change, and safe in diagnostics, whatever the session carries.

```yaml
scope: the token and the storage that hangs off it; the bounds it enforces are declared by decision:session-lifetime-owned-by-auth
token:
  entropy: at least 256 random bits
  browser_value: canonical opaque encoding
  store_key: cryptographic hash of browser value
cookie_defaults:
  http_only: true
  secure: true outside explicit loopback development
  same_site: lax
  path: /
lifetime:
  declared_by: data:authentication-runtime-config, under auth.session
  enforced_by: api:session-manager and flow:session-lifecycle
  shape: an absolute expiry and an optional idle expiry, both authoritative on the server
rules:
  - rotate after login, privilege change, and other authentication-strength changes
  - authentication-strength change includes the re-proof of policy:reauthentication, which also resets idle expiry
  - rotation changes the token and preserves every registered slot, so fixation resistance costs no application state
  - expiry is a bound on the whole session, not a substitute for the per-operation assurance of concept:assurance-axes
  - revoke server state on logout, and destroy every slot with it
  - enforce absolute expiry and optional idle expiry whenever a lifetime source declares them
  - a session with no lifetime source is bounded by the browser alone, which decision:session-lifetime-owned-by-auth accepts and reports at startup
  - protect state-changing cookie-authorized requests with policy:csrf-protection, whether or not the request is authenticated
  - never log cookie values, token hashes, stored slot data, or backend credentials
  - reject insecure SameSite none cookies
  - bind authorization to validated data:request-authentication, never cookie presence alone
  - a session existing for an anonymous browser is not an authentication claim, so it grants nothing
  - sqlite://:memory: is process-local and unsuitable for multi-process production sessions
  - a cookie-backed record is sealed under policy:cookie-value-protection and bound to the hash of its own token
  - a cookie backend cannot revoke an issued record, so a deployment that must end sessions on demand uses a server-side store
  - state that must be revocable declares session.ServerOnly rather than trusting the configured backend, per decision:slot-declared-placement
  - an anonymous session.Private slot is unrevocable while it is a cookie, which is accepted because revocation has no meaning before a login
  - promotion moves a value from the client to the server and never the reverse, so a logout hands the browser nothing back
```
