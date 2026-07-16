---
id: requirement:contrib-oauth
type: requirement
title: TinyGo OAuth Client
---
contrib/oauth implements a bounded OAuth 2.0 Authorization Code client with PKCE using requirement:contrib-auth-common and requirement:contrib-auth-state.

```yaml
package: contrib/oauth
scope: authorization code client only
public_api:
  - NewClient(config, options)
  - Config.EndpointValidator applies caller-specific trust policy to configured endpoints
  - Client.BeginAuthorization(context, options) returns URL and transaction key
  - Client.HandleCallback(context, transaction key, callback values) returns TokenSet
  - Options.TransactionValidator runs after callback state correlation and before token exchange
required:
  - generate state and PKCE verifier from crypto/rand
  - S256 PKCE only
  - store expiring correlation data and atomically consume it before code exchange
  - exact redirect URI supplied by application
  - client_secret_basic and client_secret_post token authentication
  - bounded token endpoint request and response
  - typed token response with bounded raw extension values
  - policy:oauth-security
deferred:
  - authorization server implementation
  - implicit and resource-owner password grants
  - device authorization and token exchange
  - private_key_jwt and DPoP
  - automatic refresh scheduling
standards:
  oauth: https://www.rfc-editor.org/rfc/rfc6749
  pkce: https://www.rfc-editor.org/rfc/rfc7636
  security: https://www.rfc-editor.org/rfc/rfc9700
```
