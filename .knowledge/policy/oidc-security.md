---
id: policy:oidc-security
type: policy
title: OIDC Security Policy
---
OIDC network discovery and browser state are validated against explicit trust anchors and single-use correlation values.

```yaml
issuer:
  - HTTPS required except explicit loopback development mode
  - discovered issuer must exactly equal configured issuer
  - authorization, token, JWKS, and UserInfo URLs must use HTTPS
http:
  - redirects disabled by default for discovery and JWKS
  - response size and request duration bounded
  - caller may restrict resolved IP ranges to prevent SSRF
browser:
  - policy:oauth-security state and PKCE rules apply
  - nonce generated from crypto/rand and stored with OAuth correlation data
  - state, nonce, and verifier expire and are atomically consumed once
tokens:
  - requirement:contrib-jwt policy applied
  - never log authorization code, tokens, verifier, nonce, or client secret
cache:
  - honor bounded HTTP cache lifetime
  - retain last valid JWKS only until configured stale limit
```
