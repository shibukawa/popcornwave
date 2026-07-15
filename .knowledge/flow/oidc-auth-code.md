---
id: flow:oidc-auth-code
type: flow
title: OIDC Authorization Code Flow
---
The OIDC client correlates browser authorization with state, nonce, and PKCE before accepting an ID Token.

```yaml
flow:
  trigger: application starts login through requirement:contrib-oidc
  steps:
    - id: prepare
      action: generate state, nonce, code verifier, and S256 challenge
    - id: store
      action: persist expiring single-use correlation data
    - id: authorize
      action: redirect to discovered authorization endpoint
    - id: callback
      action: require matching state and consume correlation data
    - id: exchange
      action: exchange code and verifier at token endpoint
    - id: verify
      action: verify ID Token with policy:jwt-security and policy:oidc-security
    - id: return
      output: verified subject, claims, token metadata, and optional access token
  failure:
    default: return typed error without tokens or secrets
```
