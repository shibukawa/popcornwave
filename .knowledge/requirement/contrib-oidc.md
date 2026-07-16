---
id: requirement:contrib-oidc
type: requirement
title: TinyGo OpenID Connect Client
---
contrib/oidc implements an OpenID Connect Relying Party over requirement:contrib-oauth using requirement:contrib-jwt.

```yaml
package: contrib/oidc
scope: relying party only
public_api:
  - Discover(context, issuer, options) returns Provider
  - Client.BeginAuthorization(context, options) returns URL and transaction key
  - Client.HandleCallback(context, transaction key, callback values) returns TokenSet
  - Client.VerifyIDToken(context, raw) returns IDToken
  - Client.UserInfo(context, accessToken) optional
  - Client.UserInfoWithSubject(context, accessToken, expectedSubject) optional
flow: flow:oidc-auth-code
required:
  - discovery metadata validation
  - caller-configurable endpoint host/IP trust policy
  - JWKS retrieval, cache expiry, and one refresh on unknown kid
  - concurrent refresh attempts are serialized to bound network amplification
  - requirement:contrib-oauth Authorization Code and S256 PKCE behavior
  - nonce transaction binding is validated before OAuth token exchange and again against the ID Token
  - issuer, audience, azp, exp, iat, nonce, and signature validation
  - a present `azp` claim must be a non-empty string matching the client ID
  - UserInfo responses require a non-empty sub claim and support subject binding
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
