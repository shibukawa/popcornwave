---
id: flow:devidp-user-selection
type: flow
title: Development IdP User Selection
---
requirement:contrib-devidp validates the authorization request, lets the developer pick a roster user without a password, and issues a PKCE-bound authorization code.

```yaml
flow:
  trigger: requirement:contrib-oidc redirects the browser to the devidp authorization endpoint
  steps:
    - id: validate
      action: resolve client_id, match redirect_uri exactly, require response_type code, and require an S256 code_challenge
    - id: pend
      action: store bounded expiring authorization state with client, redirect URI, challenge, nonce, scope, and state
    - id: select
      action: render ui:devidp-login with the data:devidp-config roster, or skip it when automatic login is set
    - id: bind
      action: attach the selected subject to the pending authorization and issue a single-use code
    - id: redirect
      action: redirect to the registered redirect URI with code and the unchanged state
    - id: exchange
      action: authenticate the client, verify the code verifier against the stored S256 challenge, and consume the code atomically
    - id: issue
      output: RS256 ID Token with roster claims and nonce, plus a Bearer access token scoped to the granted scopes
    - id: userinfo
      action: return the same subject and claims for a valid access token
  failure:
    validation: render an error page without redirecting when client_id or redirect_uri is unusable
    protocol: redirect a standard OAuth error to a validated redirect URI otherwise
    token: return a standard token error and consume nothing
    default: fail closed without issuing a code or token
rules:
  - a code that fails PKCE, client authentication, redirect URI, or expiry checks is destroyed rather than retried
  - replaying a consumed code invalidates nothing else but always fails
  - the selection step performs no credential check by design, which is why policy:devidp-safety confines the provider to development
  - automatic login exists for api:testutil-idp and never changes the issued token shape
```
