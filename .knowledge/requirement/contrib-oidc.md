---
id: requirement:contrib-oidc
type: requirement
title: TinyGo OpenID Connect Client
---
contrib/oidc implements an OpenID Connect Relying Party for Authorization Code flow with PKCE using requirement:contrib-jwt.

```yaml
package: contrib/oidc
scope: relying party only
public_api:
  - Discover(context, issuer, options) returns Provider
  - Client.AuthCodeURL(state, nonce, options)
  - Client.Exchange(context, code, verifier) returns TokenSet
  - Client.VerifyIDToken(context, raw) returns IDToken
  - Client.UserInfo(context, accessToken) optional
flow: flow:oidc-auth-code
required:
  - discovery metadata validation
  - JWKS retrieval, cache expiry, and one refresh on unknown kid
  - S256 PKCE
  - client_secret_basic and client_secret_post token authentication
  - issuer, audience, azp, exp, iat, nonce, and signature validation
  - exact redirect URI supplied by application
deferred:
  - OpenID Provider implementation
  - dynamic client registration
  - implicit and hybrid flows
  - JWE ID Tokens
  - private_key_jwt
security: policy:oidc-security
standard: https://openid.net/specs/openid-connect-core-1_0-18.html
```
