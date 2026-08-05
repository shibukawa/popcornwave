---
id: requirement:contrib-jwt
type: requirement
title: TinyGo JWT
---
contrib/jwt strictly parses and verifies bounded signed JWTs, validates registered claims, rejects empty or excessively numerous audience values, and offers signing only for algorithms proven by the TinyGo matrix.

```yaml
package: contrib/jwt
public_api:
  - Parse(token, options) returns Token
  - Verify(token, KeyResolver, options) returns Claims
  - Sign(header, claims, Signer) returns compact JWT
  - ParseAndVerify convenience function
  - JWKS parser and key resolver
  - NewRemoteKeySet(issuer, options) returns a fetching KeyResolver over a published key set
remote_key_set:
  added_for: requirement:jwt-only-api-authentication, which needs keys from an issuer rather than keys a caller already holds
  why_here: the package already parses and resolves a JWKS, and only a package under contrib may use the bounded HTTP and JSON helpers that make a fetch safe; requirement:contrib-oidc has an equivalent path but it is unexported and reachable only through a full relying-party client
  modes: OpenID Connect metadata, RFC 8414 authorization server metadata, or a directly supplied jwks_uri
  trust:
    - https only, except for a loopback issuer under the development allowance
    - the metadata document's own issuer must equal the configured one
    - the key set must share the issuer's scheme and host, so a metadata document cannot point the key source at another origin
    - redirects are rejected, because a redirect on a key fetch is a request to take keys from somewhere else
  freshness:
    cache: keys are refetched once the cache TTL passes
    unknown_kid: one refresh, serialized by the same lock, and no more often than the configured cooldown
    stale_on_failure: a refresh that fails leaves the previous key set serving, because the keys did not become untrustworthy when the issuer became unreachable
  deferred: discovery is not attempted until the first token needs a key, so an application starts while its authorization server is down
claims:
  registered:
    - iss
    - sub
    - aud
    - exp
    - nbf
    - iat
    - jti
  custom: raw JSON values with typed accessors
algorithms:
  verification_required:
    - HS256
    - RS256
  signing_required:
    - HS256
  signing_conditional:
    - RS256 when TinyGo crypto/rsa passes target vectors
  deferred:
    - ES256
    - EdDSA
    - JWE
security: policy:jwt-security
limits:
  - parser options have hard upper bounds for token, segment, depth, and member counts
  - HMAC signing keys and serialized signed output are bounded
standards:
  jwt: https://www.rfc-editor.org/rfc/rfc7519
  jws: https://www.rfc-editor.org/rfc/rfc7515
  best_practices: https://www.rfc-editor.org/rfc/rfc8725
```
