---
id: requirement:contrib-jwt
type: requirement
title: TinyGo JWT
---
contrib/jwt strictly parses and verifies signed JWTs, validates registered claims, and offers signing only for algorithms proven by the TinyGo matrix.

```yaml
package: contrib/jwt
public_api:
  - Parse(token, options) returns Token
  - Verify(token, KeyResolver, options) returns Claims
  - Sign(header, claims, Signer) returns compact JWT
  - ParseAndVerify convenience function
  - JWKS parser and key resolver
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
standards:
  jwt: https://www.rfc-editor.org/rfc/rfc7519
  jws: https://www.rfc-editor.org/rfc/rfc7515
  best_practices: https://www.rfc-editor.org/rfc/rfc8725
```
