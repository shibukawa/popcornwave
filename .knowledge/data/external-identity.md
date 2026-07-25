---
id: data:external-identity
type: data
title: External OIDC Identity
---
An external identity links one verified OIDC subject to one data:user-account.

```yaml
fields:
  issuer: canonical verified issuer URL
  subject: non-empty verified sub claim
  account_id: data:user-account identifier
  linked_at: timestamp
  last_authenticated_at: timestamp
  display_claims: optional non-authoritative copied claims
unique_key:
  - issuer
  - subject
rules:
  - issuer and subject come only from a verified ID Token or subject-bound UserInfo response
  - mutable email, name, and preferred_username claims do not identify the link
  - one external identity cannot link to multiple local accounts
  - token bodies and provider secrets are never persisted in this record
```
