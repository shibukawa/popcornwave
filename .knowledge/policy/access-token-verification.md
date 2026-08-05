---
id: policy:access-token-verification
type: policy
title: Access Token Verification Policy
---
A bearer access token is verified as a resource-server credential, which is a stricter question than policy:jwt-security answers alone: it must name this API, come from the one configured issuer, and not be an ID Token wearing a different hat.

```yaml
inherits: policy:jwt-security in full, over requirement:contrib-jwt
credential_extraction:
  header: Authorization, scheme Bearer, exactly one occurrence
  form: one b64token per RFC 6750, rejecting control bytes, whitespace, and any second credential
  refused_sources: query parameter and form body, because a URL-borne credential reaches logs, referrers, and history
  absent: no credential is unauthenticated, not invalid; the guard of policy:authenticated-path-protection decides what that costs
token_type:
  required: the typ header equals the configured auth.jwt.required_token_type, default at+jwt per RFC 9068
  reason: an ID Token and an access token are both signed by the same issuer with the same key, so without this check an ID Token minted for a browser login is a valid API credential
  relaxation: an empty required_token_type accepts an absent typ, for an issuer predating RFC 9068
  relaxation_cost: it is an explicit configuration act, never a fallback, and rule:configuration-advisories reports it, because the deployment has just given up the one check that separates an access token from an ID Token
  relaxation_compensation: a relaxed deployment must set an audience the issuer does not put in its ID Tokens, which is the remaining thing that keeps the two apart
issuer:
  configured: auth.jwt.issuer, an exact URL
  metadata: the issuer field of the discovered document must equal it, byte for byte after normalization
  claim: the iss claim must equal it, exactly and case-sensitively
  reason: the three-way equality is the mix-up defense; a token from another issuer whose key happens to resolve is otherwise indistinguishable
audience:
  configured: auth.jwt.audience, a non-empty list naming this API as the authorization server registered it
  match: auth.jwt.audience_match, any by default, all for an API that requires every named resource
  shapes: a string or an array of strings; every other shape is refused rather than coerced
  empty_configuration: refused at startup, because a token verified without an audience check is a token minted for some other service
  no_client_id_default: the audience is not derived from a client id, because a resource server is not the client
algorithms:
  configured: auth.jwt.algorithms, an exact allowlist, default RS256
  refused: alg none, a symmetric algorithm against a JWKS key, and any value outside the list
  selection: the key is selected by kid and the configured algorithm, never by the alg header alone
required_claims:
  always: iss, sub, aud, exp, and iat
  with_at_jwt: jti as well, which RFC 9068 requires and which policy:token-revocation needs to name a token
  reason: each is required by RFC 9068, and a token missing one is refused rather than verified under a weaker rule; an optional check is a check some deployment will find switched off after an incident
  sub: non-empty, and separately from auth.jwt.identity_claim, so a deployment keying on another claim still gets the subject the token was minted for
  nbf: validated when present, not required, because RFC 9068 does not mint it
time:
  validated: exp, iat, and nbf when present
  leeway: auth.jwt.leeway, default 30s, with a hard upper bound at startup
  future_iat: refused beyond the leeway
  lifetime_sanity: a token whose exp minus iat exceeds auth.jwt.max_token_lifetime is refused, because it outlives what the deployment declared and would outlive a subject-form entry of policy:token-revocation
  clock: injectable, per policy:jwt-security
key_discovery:
  document: auth.jwt.discovery selects the OpenID Connect metadata path, the RFC 8414 authorization server metadata path, or a jwks_uri supplied directly
  trust: the endpoint host policy, the endpoint validator contract, and the JWKS cache expiry of requirement:contrib-oidc apply unchanged
  transport: https only, so a metadata or JWKS response cannot be rewritten in flight
  same_origin: the discovered jwks_uri must share the origin of auth.jwt.issuer, because a metadata document that can point the key source at another host makes the issuer check decorative
  bounds: the JWKS document size and key count caps of policy:jwt-security, applied to the fetched document before parsing
  first_use: discovery is deferred to the first request that needs a key and is not cached on failure
  unknown_kid: one refresh, serialized across concurrent requests
  cooldown: auth.jwt.jwks_refresh_cooldown bounds how often an unknown kid may cause a refresh, so an unauthenticated stream of forged kid values cannot amplify into traffic against the issuer
  loopback: auth.jwt.allow_loopback_http relaxes the https and same-origin rules for a loopback host, development only, matching oidc.allow_loopback_http
scopes:
  field: auth.jwt.required_scopes, its own list rather than a claim rule
  reason: the scope claim is one space-delimited string per RFC 8693, and the JSON Pointer rule of policy:oidc-admission compares exact strings and arrays, so it cannot see the members
  match: every configured scope must be present
  boundary: a coarse gate for the whole API; per-route scope checking is application authorization, per data:request-authentication
bounds:
  size: auth.jwt.max_token_bytes, with a hard upper bound, applied before any parsing
  claims: the parser member, depth, and segment bounds of requirement:contrib-jwt
errors:
  categories: stable and few, such as missing, malformed, untrusted, expired, wrong_audience, and revoked
  body: api:problem-response carrying the category and nothing derived from the token
  header: WWW-Authenticate Bearer with the RFC 6750 error code, and the RFC 9470 challenge when api:assurance-guard is the one refusing
  logging: the category, the issuer, and the kid; never the compact token, a claim value, or key material
rules:
  - verification is fail-closed, and an unimplemented check is a startup refusal rather than a skipped step
  - never decode a token for a decision before its signature is verified, including for issuer selection
  - a verified claim set is copied and frozen before handler dispatch, per data:request-authentication
  - no per-token verification cache, because a cached admit outlives policy:token-revocation
```
