---
id: api:bearer-authentication
type: api
title: Bearer Authentication Surface
---
Under requirement:jwt-only-api-authentication the framework installs one middleware and publishes the revocation calls, and mounts no route at all, so an application registers nothing and writes no protocol code.

```yaml
package: github.com/shibukawa/popcornweb/plugin/auth
registration:
  mechanism: importing the package installs the extensions through api:framework-extension, as api:authentication-endpoints already describes
  selection: auth.mode jwt_only decides that the bearer middleware is installed and that no endpoint is
  application_wiring: auth.SetAccountResolver, optional here per policy:bearer-admission
endpoints:
  mounted: none
  login: absent; a machine client authenticates at its own issuer
  callback: absent; there is no redirect to come back from
  logout: absent; a credential this framework never issued is not one it can end, and policy:token-revocation is the operator path instead
  revocation: no HTTP surface, for the reason policy:token-revocation gives
middleware:
  placement: inside every framework middleware, and before policy:authenticated-path-protection, matching the placement rule of api:authentication-endpoints
  action: extract the credential, run policy:access-token-verification, then policy:bearer-admission, then policy:token-revocation, and publish data:request-authentication
  absent_credential: leaves the request unauthenticated, so a public path still serves
  csrf:
    resolution: startup refuses security.csrf.enabled in this mode rather than exempting the request
    why_not_exempt: an exemption keyed on the Authorization header would be a CSRF bypass in any deployment that also authenticates by cookie, because the attacker supplies the header and the browser supplies the cookie
    why_refusing_is_free: the check needs a secret held in a session slot and this mode creates no session, so leaving the pair configured would refuse every POST at runtime with a message about a missing session, a long way from the setting that caused it
  refusal: one enumeration-safe 401 with an identical body for every cause, and a WWW-Authenticate Bearer challenge naming the audience as the realm, which the caller already had to know to obtain a token
surface:
  - auth.SetAccountResolver(resolver) links a verified identity to an application account, the same seam every other mode uses
  - auth.Bearer(context) returns the verified caller as data:request-authentication already carries it, holding the account, the account summary, the verified identity and its claims, and the token times
  - auth.BearerClaims(context) returns the frozen verified claim set alone, for a deployment authorizing from claims rather than from an account
  - auth.RevokeToken(context, issuer, tokenID, note) writes the token form of data:revoked-token-record
  - auth.RevokeSubject(context, issuer, identityKey, note) writes the subject form
  - auth.ReinstateToken and auth.ReinstateSubject remove an entry, for a revocation issued in error
  - auth.TokenRevoked and auth.SubjectRevoked report the stamp and presence, reading past the request cache, for an administrative view that must not guess
  - auth.MigrationSQL() publishes popcornweb_auth_revocation beside the tables api:authentication-endpoints already publishes
signature_notes:
  issuer_is_explicit: every revocation call names the issuer rather than assuming the configured one, because the entry is scoped by it and a call that inferred it would silently write to the wrong scope if the deployment ever gained a second issuer
  expiry_is_derived: the caller supplies no expiry; it is revoked_at plus auth.jwt.max_token_lifetime, which the mode already requires, so an operator cannot write an entry that expires before the tokens it must refuse
  no_user_call: auth.User returns the session summary of a ceremony mode and stays absent here, because this mode establishes no session; auth.Bearer is its counterpart
absent_surface:
  - auth.Session, because this mode establishes none
  - the passkey and bootstrap calls, which belong to modes that enroll a credential
  - anything that mints, refreshes, or exchanges a token, per the non_goals of requirement:jwt-only-api-authentication
guard:
  paths: auth.protection.include, unchanged from policy:authenticated-path-protection
  unauthenticated: unauthorized only; redirect is refused at startup, because there is nowhere to send a client the framework did not authenticate
  assurance: api:assurance-guard works, with both wrappers answering 401 and no step-up, per requirement:jwt-only-api-authentication
testing:
  seam: api:testutil-auth, which sets an authenticated request without a provider, so a handler test proves the admit and refuse paths with no issuer running
  extension: the same seam supplies claims and a revoked identifier, per decision:test-authentication-seams
rules:
  - the surface a handler calls is data:request-authentication and auth.User, which is what every other mode gives it, so application code does not branch on the mode
  - a revocation call is an application or operator action; the framework never revokes on its own
  - errors reaching a client are api:problem-response with the stable categories of policy:access-token-verification
implementation_state:
  verified: 2026-08-14, every call above present in plugin/auth and wired through setupBearer
  storage_narrowed: relational only; see the shipped storage of policy:token-revocation
  tested: the store and the refusal path directly, and RevokeToken end to end; the RevokeSubject, reinstate, and administrative-read wrappers have no test of their own and no example calls them
  documented: 2026-08-14, guides/backend/token-revocation in both locales covers the revoke, reinstate, and administrative-read calls, the two forms, on_unavailable, and the propagation-delay cache, linked from the jwt_only section of the authentication guide
```
