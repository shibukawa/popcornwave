---
id: requirement:contrib-oauth
type: requirement
title: TinyGo OAuth Client
---
contrib/oauth implements bounded OAuth 2.0 Authorization Code with PKCE and Device Authorization clients using requirement:contrib-auth-common and requirement:contrib-auth-state.

```yaml
package: contrib/oauth
scope: authorization code and RFC 8628 device authorization clients
public_api:
  - NewClient(config, options)
  - Config.EndpointValidator applies caller-specific trust policy to configured endpoints
  - Client.BeginAuthorization(context, options) returns URL and transaction key
  - Client.HandleCallback(context, transaction key, callback values) returns TokenSet
  - Options.TransactionValidator runs after callback state correlation and before token exchange
  - TransactionCodec implements api:auth-state-codec
  - NewDeviceClient(config, options) returns a client that needs no redirect URI or state store
  - DeviceClient.Begin(context, options) returns data:device-authorization
  - DeviceClient.Poll(context, authorization) waits until TokenSet or a terminal typed error
required:
  - generate state and PKCE verifier from crypto/rand
  - S256 PKCE only
  - store expiring correlation data and atomically consume it before code exchange
  - exact redirect URI supplied by application
  - protocol-managed authorization parameters cannot be overridden by extension parameters
  - scope values follow the RFC scope-token grammar and cannot contain whitespace or control bytes
  - endpoint validator mutations cannot rewrite the configured request endpoints
  - client_secret_basic and client_secret_post token authentication
  - public-client authentication with client_id and no client secret only on the typed device client
  - device authorization request, response, polling, and errors follow flow:oidc-device-authorization
  - polling timing, cancellation, backoff, and terminal behavior follow policy:device-authorization-security
  - bounded token endpoint request and response
  - bounded standard token string fields
  - typed token response with bounded raw extension values
  - policy:oauth-security
deferred:
  - authorization server implementation
  - implicit and resource-owner password grants
  - OAuth token exchange grant
  - private_key_jwt and DPoP
  - automatic refresh scheduling
standards:
  oauth: https://www.rfc-editor.org/rfc/rfc6749
  pkce: https://www.rfc-editor.org/rfc/rfc7636
  security: https://www.rfc-editor.org/rfc/rfc9700
  device_grant: https://www.rfc-editor.org/rfc/rfc8628
```
