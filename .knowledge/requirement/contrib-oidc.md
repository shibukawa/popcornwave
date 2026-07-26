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
  - Provider.EndSessionEndpoint() reports the discovered RP-initiated logout endpoint
  - Client.EndSessionURL(options) builds a logout request, or an empty string when the provider advertises none
flow: flow:oidc-auth-code
required:
  - discovery metadata validation
  - caller-configurable endpoint host/IP trust policy
  - endpoint trust policy applies to authorization, token, JWKS, and UserInfo metadata endpoints
  - endpoint validator errors are normalized without exposing caller error strings
  - endpoint validators receive a copy and cannot rewrite the URL used for requests
  - JWKS retrieval, cache expiry, and one refresh on unknown kid
  - concurrent refresh attempts are serialized to bound network amplification
  - requirement:contrib-oauth Authorization Code and S256 PKCE behavior
  - nonce transaction binding is validated before OAuth token exchange and again against the ID Token
  - issuer, audience, azp, exp, iat, nonce, and signature validation
  - a present `azp` claim must be a non-empty string matching the client ID
  - ID Token parser size options have hard upper bounds at client construction
  - UserInfo responses require a non-empty sub claim and support subject binding
  - bearer access tokens reject control and whitespace bytes before header use
  - token exchange accepts only the Bearer token type used by the OIDC UserInfo flow
  - exact redirect URI supplied by application
  - end_session_endpoint is validated like every other discovered endpoint
  - id_token_hint, post_logout_redirect_uri, and state are bounded and rejected when malformed
deferred:
  - OpenID Provider implementation beyond the development-only requirement:contrib-devidp
  - public clients without a configured client secret
  - dynamic client registration
  - implicit and hybrid flows
  - JWE ID Tokens
  - private_key_jwt
security: policy:oidc-security
standards:
  core: https://openid.net/specs/openid-connect-core-1_0-18.html
  rp_initiated_logout: https://openid.net/specs/openid-connect-rpinitiated-1_0.html
```
