---
id: flow:bearer-request-authentication
type: flow
title: Bearer Request Authentication Flow
---
Every request under requirement:jwt-only-api-authentication runs the same ordered checks, cheapest and most conclusive first, so a forged credential is refused before it costs a key fetch or a store read.

```yaml
flow:
  trigger: a client sends a request carrying an Authorization Bearer credential
  steps:
    - id: extract
      action: read the single Bearer credential per policy:access-token-verification
      absent: publish an unauthenticated data:request-authentication and continue; the guard decides
      malformed: refuse with the malformed category
    - id: bound
      action: reject a credential over auth.jwt.max_token_bytes before parsing it
    - id: parse
      action: parse the compact form under the requirement:contrib-jwt bounds, without trusting any claim yet
    - id: dev-branch
      action: when every lock of policy:dev-token-relaxation is open, take the relaxed path, which skips resolve-key and verify and rejoins at admit
      otherwise: absent from the binary, per the build lock of that policy
    - id: resolve-key
      action: select the key by kid and the configured algorithm from the cached JWKS
      unknown_kid: refresh once, serialized and rate-limited by auth.jwt.jwks_refresh_cooldown
      first_use: discover the issuer metadata now, and do not cache a failed discovery
    - id: verify
      action: check signature and alg allowlist, then typ, iss, aud, sub, exp, iat, jti, nbf when present, and the exp-minus-iat lifetime bound, per policy:access-token-verification
      missing_required_claim: refused; the required set is not weakened by an issuer that omits one
    - id: admit
      action: evaluate policy:bearer-admission against the verified claims
      failure: refuse without creating or linking an account
    - id: revoke-check
      action: look up the token and subject forms of data:revoked-token-record, when auth.jwt.revocation.enabled
      revoked: refuse with the revoked category
      unavailable: answer 503 per policy:token-revocation, rather than admitting
    - id: resolve-account
      action: call the auth.SetAccountResolver seam when one is configured
      absent_resolver: skip, leaving auth.User empty and the claims still available
    - id: publish
      output: data:request-authentication with method bearer, the frozen claim set, and the token expiry as expires_at
    - id: guard
      action: policy:authenticated-path-protection admits the request or answers 401
    - id: assure
      action: api:assurance-guard evaluates a per-route requirement against auth_time or iat, answering 401 with the RFC 9470 challenge
  ordering_reason:
    - verification before admission, so an allowlist lookup is never spent on a forged token
    - admission before revocation, so a store read is never spent on a token that would be refused anyway
    - revocation before account resolution, so a revoked token never reaches application code
  failure:
    response: api:problem-response with a stable category and a WWW-Authenticate Bearer header
    uniformity: expired, revoked, and wrong-audience refusals are equally uninformative to the client and fully distinguished in the log
    no_side_effects: a refused request writes no record, creates no account, and touches no store
```
