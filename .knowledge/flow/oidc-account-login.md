---
id: flow:oidc-account-login
type: flow
title: OIDC Account Login
---
OIDC login verifies an external IdP identity, resolves or provisions a local account, and creates a Popcorn Wave session.

```yaml
flow:
  - begin requirement:contrib-oidc authorization code flow with PKCE, state, and nonce
  - consume callback state through requirement:contrib-auth-state
  - verify token signature, issuer, audience, nonce, time, and subject
  - apply policy:oidc-admission to the verified identity and claims
  - resolve data:external-identity by canonical issuer and subject
  - provision data:user-account and external link only when admission, auto-provision, and registration policy all permit
  - reject suspended, deleted, ambiguous, or conflicting accounts
  - create or rotate api:session-manager session
  - record data:request-authentication method as OIDC
  - optionally offer flow:passkey-enrollment
rules:
  - policy:account-linking governs existing-account links
  - deny admission before creating, linking, or mutating an account
  - access and refresh tokens are not stored in the login session unless application functionality explicitly requires them
```
