---
id: requirement:contrib-devidp
type: requirement
title: Development OpenID Provider
---
contrib/devidp is a development-only OpenID Provider that authenticates by selecting a user from a TOML roster, giving requirement:contrib-oidc an in-repo counterparty without external IdP credentials.

```yaml
package: contrib/devidp
scope: development and test identity provider only
runtime: host Go only, exempt from the policy:contrib-compatibility TinyGo matrix per decision:devidp-scope-reduction
origin: system:oidcld
public_api:
  - LoadConfig(path) and ParseConfig(source, base) return data:devidp-config
  - New(config, options) returns Provider
  - Start(context, addr, config, options) listens on loopback, derives the issuer from the port, and returns Server
  - Provider.Handler() returns an http.Handler for the issuer base path
  - Provider.Issuer() and Provider.Endpoint(path) return absolute URLs
  - Provider.RegisterClient(spec) returns generated client credentials for a tool-owned ephemeral client
  - Provider.SetLoginUser(subject) and Provider.LoginUser() control automatic login
  - Provider.Users() returns the roster in login-screen order
  - Provider.Reload(config) swaps roster and scopes while the issuer, clients, and signing key survive
  - Provider.Close() releases signing keys and pending authorization state
endpoints:
  - GET /.well-known/openid-configuration
  - GET /jwks.json
  - GET and POST /authorize
  - POST /token
  - GET /userinfo
  - GET and POST /end_session
  - GET and POST /login serving ui:devidp-login
configuration: data:devidp-config
flow: flow:devidp-user-selection
required:
  - Authorization Code grant only, with S256 PKCE mandatory for every client
  - client_secret_basic and client_secret_post token authentication, because requirement:contrib-oauth defers public clients
  - RS256 signing with the key published at the JWKS endpoint
  - discovery advertises only implemented grants, response types, scopes, claims, and the S256 challenge method
  - discovery issuer exactly equals the configured issuer so policy:oidc-security issuer checks pass
  - exact match of the redirect URI against the registered list of the requesting client, except the loopback relaxation policy:devidp-safety grants to ephemeral clients
  - clients declared in data:devidp-config and clients registered by the running tool share one validation path
  - the issuer, and therefore every discovery URL, is resolved after the listener port is known
  - authorization codes are single-use, short-lived, and bound to client, redirect URI, PKCE challenge, nonce, and subject
  - nonce is echoed into the ID Token whenever the authorization request supplies one
  - state is returned unchanged and is never interpreted
  - granted scope is the intersection of the request, the configured valid scopes, and the per-user extra scopes
  - openid, profile, and email claims are mapped from the roster entry and repeated by UserInfo
  - UserInfo requires a Bearer access token issued by this provider and returns a matching sub
  - automatic login mode issues the code without rendering ui:devidp-login
  - RP-initiated logout verifies id_token_hint against the provider signing key before ending anything
  - logout revokes the access tokens of that subject and client, so a token cannot outlive the session
  - the post_logout_redirect_uri needs no registration and may be any local URL, because requiring one would restore the friction this provider exists to remove
  - a non-local post_logout_redirect_uri is refused, so the provider cannot become an open redirect off the machine
  - loopback matching accepts loopback addresses, localhost, and any RFC 6761 name under .localhost
  - state is returned unchanged to the post-logout URI
  - bounded request parsing, bounded pending-authorization count, and expiry sweeping
  - standard OAuth and OIDC error responses for every rejection
  - policy:devidp-safety guardrails
deferred:
  - local certificate authority and TLS termination
  - reverse proxy, static hosting, and mock APIs
  - device authorization and client credentials grants
  - EntraID v1 and v2 compatibility modes
  - refresh tokens and back-channel or front-channel logout notification
  - developer console and MCP surfaces
  - public clients, private_key_jwt, and DPoP
  - passwords, MFA, consent screens, and account registration
  - dynamic client registration
  - implicit and hybrid flows
  - persisted tokens, codes, or signing keys
consumers:
  - api:cli-dev
  - api:testutil-idp
  - requirement:contrib-oidc fixtures under requirement:contrib-acceptance
dependencies:
  - requirement:contrib-auth-common for crypto/rand secrets, constant-time comparison, and S256 challenge derivation
  - system:tinybind minitoml for data:devidp-config parsing
signing:
  location: contrib/devidp signs RS256 itself
  reason: requirement:contrib-jwt signs HS256 only because RS256 signing waits on the TinyGo matrix, and this host-only provider must not widen that surface
  verification: relying parties verify through requirement:contrib-jwt JWKS parsing, which already supports RS256
standards:
  oidc: https://openid.net/specs/openid-connect-core-1_0-18.html
  discovery: https://openid.net/specs/openid-connect-discovery-1_0.html
  pkce: https://www.rfc-editor.org/rfc/rfc7636
  rp_initiated_logout: https://openid.net/specs/openid-connect-rpinitiated-1_0.html
```
