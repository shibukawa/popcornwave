---
id: policy:oauth-security
type: policy
title: OAuth Client Security Policy
---
OAuth browser correlation and token exchange fail closed on state, PKCE, endpoint trust, redirect URI, and bounded secret handling.

```yaml
browser:
  - state and PKCE verifier come from crypto/rand
  - state has at least 128 bits and is compared exactly
  - correlation data expires and is consumed atomically before token exchange
  - protocol-specific transaction validation runs after correlation checks and before token exchange
  - authorization error callbacks require the same state validation as successful callbacks
endpoints:
  - HTTPS required except explicit loopback development mode
  - outbound transport follows policy:outbound-transport-security
  - authorization and token endpoints come from caller configuration or validated discovery
  - caller-specific endpoint trust validation may restrict configured hosts or resolved IP ranges
  - redirects disabled by default for token requests
  - response size and request duration bounded
tokens:
  - never log authorization code, access token, refresh token, verifier, state, or client secret
  - reject duplicate security-relevant response members
  - stable errors contain no secret values
storage: requirement:contrib-auth-state
```
