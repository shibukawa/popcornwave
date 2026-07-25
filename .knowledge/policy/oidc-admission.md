---
id: policy:oidc-admission
type: policy
title: OIDC Admission Policy
---
OIDC verification proves identity; admission separately decides whether that identity may enter this application.

```yaml
modes:
  existing:
    admit: canonical issuer and subject already resolve to data:external-identity
    provisioning: forbidden
  claim:
    admit: configured verified claim rule matches
    provisioning: controlled by oidc.auto_provision and registration policy
  authenticated:
    admit: every identity verified by the configured issuer
    provisioning: controlled by oidc.auto_provision and registration policy
claim_rule:
  path: JSON Pointer into the selected verified claim set
  values: non-empty allowlist of exact string values
  match:
    any: scalar equals or array intersects the allowlist
    all: array contains every configured value
failure:
  - reject missing paths, unexpected value types, and non-matches
  - return an enumeration-safe access-denied response
  - do not create or link an account
rules:
  - evaluate only after issuer, audience, signature, nonce, time, and subject verification
  - never use unverified request parameters or decoded-but-unverified tokens
  - apply admission on every OIDC login, including existing accounts
  - treat claim matching as exact and case-sensitive
  - avoid a general expression language in framework configuration
  - audit rule outcome without raw tokens, secrets, or unnecessary claim values
boundary:
  - OIDC admission governs OIDC login and provisioning, not arbitrary application authorization
  - passkey login authenticates the local account without refreshing IdP claims
  - deployments requiring continuing organization membership must define entitlement refresh, expiry, and revocation beyond this admission gate
```
