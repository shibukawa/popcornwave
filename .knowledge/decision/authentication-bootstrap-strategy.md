---
id: decision:authentication-bootstrap-strategy
type: decision
title: Authentication Bootstrap Strategy
---
Popcorn Web supports OIDC-backed account bootstrap with optional passkey login as the recommended default, plus explicit passkey-only deployments.

```yaml
modes:
  oidc_passkey:
    bootstrap: flow:oidc-account-login
    convenience_login: flow:passkey-login
    enrollment: flow:passkey-enrollment
    recovery: trusted OIDC reauthentication or administrator policy
    recommendation: default
  oidc_only:
    bootstrap_and_login: flow:oidc-account-login
  passkey_only:
    bootstrap: flow:passkey-only-registration
    issued_proof: data:account-bootstrap-credential
    login: flow:passkey-login
    recovery: explicit policy:account-recovery without implicit IdP fallback
common:
  account: data:user-account
  external_identity: data:external-identity
  passkey_credential: data:passkey-credential
  session: api:session-manager
  request_result: data:request-authentication
  linking: policy:account-linking
  recovery: policy:account-recovery
  bootstrap_security: policy:bootstrap-credential-security
  oidc_admission: policy:oidc-admission
boundaries:
  - application owns account and credential persistence
  - framework does not require a repository type or fixed database schema
```
