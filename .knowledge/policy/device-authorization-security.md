---
id: policy:device-authorization-security
type: policy
title: Device Authorization Security Policy
---
Device authorization treats the constrained application as a public client and protects the human code, polling channel, and approval ceremony independently.

```yaml
client:
  - embedded client secrets are not credentials; public clients send client_id and no secret
  - confidential authentication is used only when a deployment provisions a secret outside the device image
  - device_authorization_endpoint and token endpoint follow policy:outbound-transport-security and policy:oidc-security host restrictions
codes:
  - device_code uses requirement:contrib-auth-common high-entropy randomness and constant-time comparison
  - user_code uses an unambiguous, case-insensitive human alphabet with formatting ignored on input
  - both codes expire together, bind client and requested scopes, and reach one terminal result
  - approval, denial, token issuance, and expiry cannot be reversed or replayed
provider:
  - rate-limit user-code guesses per source and per pending transaction
  - enforce the advertised poll interval and return slow_down to early or excessive pollers
  - bound pending records globally and per client, sweep expiry, and reveal no record existence beyond protocol responses
approval:
  - show user_code, client identity, requested scopes, and a warning that another device will receive access
  - require explicit approve or deny after roster selection; verification_uri_complete never approves automatically
  - never display device_code in the browser
tokens:
  - issue only after approval to the same client that owns device_code
  - return no refresh token in the initial scope
  - never log device_code, access token, refresh token, ID Token, or confidential client secret
```
