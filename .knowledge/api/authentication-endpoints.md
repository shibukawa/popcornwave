---
id: api:authentication-endpoints
type: api
title: Authentication Endpoints
---
The runtime mounts login, callback, and logout from data:authentication-runtime-config, so an application registers no authentication route and writes no protocol code.

```yaml
package: github.com/shibukawa/popcornwave/plugin/auth
registration:
  mechanism: importing the package installs session, authentication, and guard extensions through api:framework-extension
  precedent: decision:import-registered-session-plugins
  application_wiring: auth.SetAccountResolver, which the import comes with
  scaffold: api:cli-init writes the resolver, the framework migrations, and the configuration for an OIDC mode
surface:
  - auth.SetAccountResolver(resolver) links a verified identity to an application account
  - auth.User(context) returns the stored account summary of the request
  - auth.Session(context) returns the validated session view
  - auth.MigrationSQL() and rdb.MigrationSQL(table) publish the framework tables
endpoints:
  login:
    path: auth.login_path, default /auth/login
    method: GET
    action: begin requirement:contrib-oidc authorization and store the transaction key, with the return path, in a short-lived cookie
  callback:
    path: auth.callback_path, default /auth/callback
    method: GET
    action: consume the transaction, verify the ID Token, and establish the session
  logout:
    path: auth.logout_path, default /auth/logout
    method: POST only
    reason: a logout reachable by link or prefetch is a denial-of-service surface
    action: delete the server-side session, then end the provider session before returning to the landing path
    scope: auth.oidc.logout_scope selects reconfirm or global, defaulting to reconfirm, per policy:provider-session-scope; the block below describes the removed bool it replaces
    provider_logout:
      default: enabled through auth.oidc.provider_logout
      reason: clearing only the local cookie leaves the provider signed in, so the next login silently returns the same user and the sign-out appears to do nothing
      request: RP-initiated logout with client_id and a post_logout_redirect_uri derived from this origin
      no_hint: the session payload holds no token body, so no id_token_hint is available or sent
      fallback: local logout when the provider advertises no end session endpoint, discovery fails, or the request cannot be built
      opt_out: auth.oidc.provider_logout false, for a provider shared with applications that must stay signed in
      resolved: policy:provider-session-scope replaces this bool with logout_scope, adds reconfirm as the default, and drops the false branch entirely; the no_hint cost above is one of its arguments
placement:
  inside: every framework middleware, so recovery, logging, and security headers apply
  resolve: a middleware installs the identity of an existing session on every request
session:
  form: opaque cookie token over a server-side record, per api:session-manager
  payload: the plugin/auth slot of api:session-registry, holding an account summary with no token body and no provider secret
  store: sessionstore/sqlite, verified at startup against rule:framework-owned-tables
  lifetime: auth.session.ttl absolute and auth.session.idle_timeout inactivity, per decision:session-lifetime-owned-by-auth
  rotation: login rotates the token, which revokes whatever the browser held before and preserves every other slot
  logout: destroys every slot, per flow:session-lifecycle
rules:
  - an unknown or expired session cookie yields an anonymous request rather than an error
  - a cross-origin logout is refused even though SameSite already blocks the cookie
  - a return path is accepted only as a rooted same-site path, so login cannot become an open redirect
  - provider errors are logged and returned as a generic unauthorized response
  - the account link is the issuer plus the claim auth.oidc.identity_claim names, never an email address
  - identity carries proof of authentication only; authorization stays with the application
modes:
  oidc_only: implemented; this concept is its whole endpoint surface
  oidc_passkey: these endpoints plus the login and enrollment endpoints of api:passkey-endpoints
  passkey_only: api:passkey-endpoints alone; login_path, callback_path, and the OIDC configuration are absent
  jwt_only: no endpoint at all, per api:bearer-authentication; the mode installs a middleware and nothing this concept describes exists in it
  logout: shared by every browser mode, because a session is mode-neutral once created; jwt_only has none, because a credential this framework never issued is not one it can end
  selection: data:authentication-runtime-config mode_validation decides which endpoints mount and which fields are read
  status: every mode serves; api:cli-init still records passkey_only with auth.enabled false until its scaffold exists
guard:
  paths: auth.protection.include requires a session, everything else stays public
  unauthenticated: redirect through the login and return, or answer 401
security: policy:oauth-security and policy:oidc-security through requirement:contrib-oidc
```
