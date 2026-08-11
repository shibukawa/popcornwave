---
id: flow:oidc-device-authorization
type: flow
title: OIDC Device Authorization Flow
---
The constrained client starts RFC 8628 authorization, presents instructions, and polls until a separately authenticated user approves, denies, or lets the request expire.

```yaml
flow:
  trigger: user starts requirement:oidc-device-authorization on the constrained device
  steps:
    - id: request
      actor: constrained device
      action: POST client_id and requested scopes to the discovered device_authorization_endpoint
    - id: issue
      actor: authorization server
      output: data:device-authorization containing device_code, user_code, verification URIs, expiry, and polling interval
    - id: present
      actor: constrained device
      action: show verification_uri and user_code; optionally expose verification_uri_complete without hiding the textual values
    - id: verify
      actor: user on separate browser device
      action: enter or confirm user_code, authenticate, inspect client and scopes, then approve or deny
    - id: poll
      actor: constrained device
      action: POST the device-code grant to the token endpoint no faster than the effective interval
    - id: verify-token
      actor: requirement:contrib-oidc
      action: verify the returned ID Token through policy:oidc-security without browser nonce correlation
    - id: return
      output: bounded TokenSet and verified IDToken
  failure:
    authorization_pending: wait the current interval and continue
    slow_down: increase every later interval by at least five seconds and continue
    timeout: apply increasing backoff before retrying within the absolute expiry
    access_denied: stop without tokens
    expired_token: stop without automatically starting another authorization
    other: stop without tokens or secrets
rules:
  - the device initiates only after user action, never automatically at application startup
  - cancellation and absolute expiry stop both waiting and network activity
  - only authorization_pending and slow_down permit continued polling
  - device_code is never displayed; user_code is never sent to the token endpoint
```
