---
id: api:authentication-endpoints
type: api
title: Authentication Endpoints
---
The runtime mounts login, callback, and logout from data:authentication-runtime-config, so an application registers no authentication route and writes no protocol code.

```yaml
package: github.com/shibukawa/popcornwave/auth
registration:
  mechanism: importing the package registers a provider through pw.RegisterAuthProvider
  precedent: decision:import-registered-session-plugins
  failure: auth.enabled with no registered provider fails startup and names the missing import
  scaffold: api:cli-init writes the blank import for an OIDC mode
surface:
  - pw.RegisterAuthProvider(AuthProvider)
  - pw.CurrentUser(context) returns data:request-authentication identity and presence
  - pw.WithIdentity(context, identity) is the provider contract, not application API
  - auth.New(AuthConfig, SessionConfig) returns the handlers for a caller mounting them elsewhere
endpoints:
  login:
    path: auth.login_path, default /auth/login
    method: GET
    action: begin requirement:contrib-oidc authorization and store the transaction key in a short-lived cookie
  callback:
    path: auth.callback_path, default /auth/callback
    method: GET
    action: consume the transaction, verify the ID Token, and establish the session
  logout:
    path: auth.logout_path, default /auth/logout
    method: POST only
    reason: a logout reachable by link or prefetch is a denial-of-service surface
    action: clear the session and transaction cookies, then end the provider session before returning to auth.post_logout_redirect
    provider_logout:
      default: enabled through auth.oidc.provider_logout
      reason: clearing only the local cookie leaves the provider signed in, so the next login silently returns the same user and the sign-out appears to do nothing
      request: RP-initiated logout with id_token_hint, client_id, and a post_logout_redirect_uri derived from this origin
      fallback: local logout when the provider advertises no end session endpoint, discovery fails, or the request cannot be built
      opt_out: auth.oidc.provider_logout false, for a provider shared with applications that must stay signed in
placement:
  inside: every framework middleware, so recovery, logging, and security headers apply
  resolve: a middleware installs the identity of an existing session on every request
session:
  form: signed cookie carrying issuer, subject, name, email, selected claims, and the ID Token used as the logout hint
  id_token: never reaches a handler and is dropped instead of the session when the cookie would exceed its bound
  signature: HMAC over the payload with session.secret, which is required
  attributes: HttpOnly, SameSite=Lax, Secure on TLS or forwarded HTTPS, Path=/
  lifetime: session.ttl
  store: none; a shared store belongs to a session backend
  bound: an oversized or unsigned cookie is rejected before parsing
rules:
  - an unusable, expired, or forged session cookie yields an anonymous request rather than an error
  - a cross-origin logout is refused even though SameSite already blocks the cookie
  - redirect targets are validated as same-origin absolute paths at startup
  - the redirect URI follows the request origin when auth.oidc.redirect_url is empty
  - provider errors are logged and returned as a generic unauthorized response
  - identity carries proof of authentication only; authorization stays with the application
modes:
  oidc: implemented
  oidc_passkey: OIDC login only until passkey enrollment exists
  passkey_only: rejected by this provider, and api:cli-init scaffolds it disabled
security: policy:oauth-security and policy:oidc-security through requirement:contrib-oidc
```
