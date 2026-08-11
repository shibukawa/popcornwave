---
id: requirement:oidc-device-authorization
type: requirement
title: OIDC Device Authorization
---
TinyGo applications on browserless or input-constrained devices obtain OIDC tokens through RFC 8628 while the user authorizes on a separate browser-capable device.

```yaml
outcome:
  device:
    - needs outbound HTTP only; no browser, redirect listener, redirect URI, or api:auth-state-codec
    - displays the verification URI and user code, then polls under server-directed timing
  user: authenticates and explicitly approves or denies the named client and scopes on another device
  token: requirement:contrib-oidc returns the same bounded TokenSet and verified IDToken shapes as flow:oidc-auth-code
scope:
  client: requirement:contrib-oauth and requirement:contrib-oidc
  development_provider: requirement:contrib-devidp
  protocol: flow:oidc-device-authorization under policy:device-authorization-security
required:
  - public clients identified by client_id without a client secret
  - confidential clients remain supported when explicitly configured
  - discovered device_authorization_endpoint is validated by policy:oidc-security
  - openid is always requested by the OIDC wrapper
  - ID Token verification binds issuer, audience, azp, signature, time, and subject
  - an absent nonce is accepted only by the typed device-flow completion path; flow:oidc-auth-code keeps mandatory nonce correlation
  - context cancellation interrupts authorization requests, polling waits, and token requests
  - attacker-controlled requests and responses, polling duration, and allocations are bounded
  - policy:contrib-compatibility host Go and TinyGo linux amd64 gates cover the client
  - requirement:contrib-acceptance exercises requirement:contrib-oidc against requirement:contrib-devidp without external credentials
deferred:
  - refresh-token scheduling and persistence
  - client credentials, token exchange, CIBA, PAR, DPoP, and dynamic client registration
  - QR generation, NFC, Bluetooth, or other presentation transports; callers receive verification_uri_complete as data
  - production OpenID Provider implementation
standards:
  device_grant: https://www.rfc-editor.org/rfc/rfc8628
  oidc_profile: OpenID Connect ID Token issuance over the OAuth device authorization grant; provider interoperability must be tested because OIDC Core does not define this grant
```
