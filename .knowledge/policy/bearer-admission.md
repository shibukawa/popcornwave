---
id: policy:bearer-admission
type: policy
title: Bearer Admission Policy
---
policy:access-token-verification proves the token; admission separately decides whether the identity inside it may use this API, which is where a deployment says only this organization enters.

```yaml
relation: the claim_rule of policy:oidc-admission, applied to a verified access token instead of to a verified ID Token
modes:
  authenticated:
    admit: every identity the configured issuer verified
    fits: an internal API whose issuer only mints tokens for people already entitled to it
  claim:
    admit: the configured claim rule matches
    fits: one issuer serving several tenants, departments, or applications
  existing:
    admit: the identity already resolves to an account through auth.SetAccountResolver
    provisioning: forbidden
  registered:
    admit: a verified claim matches a row registered in advance for this issuer
    storage: the popcornweb_auth_allowlist table of policy:oidc-admission, under rule:framework-owned-tables
    lookup_failure: an error, never a denial, so an outage cannot silently change who may enter
claim_rule:
  definition: policy:oidc-admission claim_rule, unchanged
  fields: auth.jwt.claim.path, auth.jwt.claim.values, and auth.jwt.claim.match
  source: the verified access token claim set only; a UserInfo call is not made, because an API request has no user to keep waiting
provisioning:
  default: none; a token is not an enrollment
  reason: an account created by an API call carries no consent, no recovery authority, and no policy:account-recovery answer, which decision:authentication-bootstrap-strategy requires of every other mode
  opt_in: auth.jwt.auto_provision, refused unless the resolver and a registration policy are both configured
identity_key:
  claim: auth.jwt.identity_claim, defaulting to sub, with the stability and value-shape contract of data:external-identity
  missing: deny; never fall back to another claim, which would admit two people as one
rules:
  - evaluate only after issuer, audience, algorithm, token type, signature, and time verification succeed
  - evaluate before policy:token-revocation, so a store lookup is spent only on a token that would otherwise be admitted
  - treat claim matching as exact and case-sensitive
  - refuse a missing path, an unexpected value shape, and a non-match alike, with one enumeration-safe response
  - avoid a general expression language in framework configuration, for the reason policy:oidc-admission gives
  - audit the outcome and the rule name without raw tokens or unnecessary claim values
boundary:
  - admission is the gate on the API as a whole; per-route, per-object, and per-tenant authorization stays with the application
  - a deployment requiring continuing organization membership sets a short token lifetime at its issuer, because admission re-runs only when a new token arrives
```
