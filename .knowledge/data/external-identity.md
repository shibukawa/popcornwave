---
id: data:external-identity
type: data
title: External OIDC Identity
---
An external identity links one verified OIDC identity to one data:user-account, through the claim a deployment selects as its account key.

```yaml
fields:
  issuer: canonical verified issuer URL
  key_claim: verified claim that identifies the account, selected by auth.oidc.identity_claim
  key: non-empty verified value of that claim
  subject: non-empty verified sub claim
  account_id: data:user-account identifier
  linked_at: timestamp
  last_authenticated_at: timestamp
  display_claims: optional non-authoritative copied claims
unique_key:
  - issuer
  - key_claim
  - key
identity_claim:
  default: sub, the only claim OpenID Connect guarantees stable and unique per issuer
  alternative: a directory-issued identifier such as an employee number, which a deployment can know before a first login
  requirement: stable for the life of the account and unique within the issuer
  value_shapes: a string, or an integer used as its literal text; every other shape is refused rather than normalized
  missing_claim: deny the login instead of falling back to the subject, which would create a second account for the same person
  change_cost: changing the selected claim orphans every account linked by the previous one
rules:
  - issuer, subject, and the selected key claim come only from a verified ID Token or subject-bound UserInfo response
  - mutable email, name, and preferred_username claims do not identify the link unless a deployment selects one and accepts its stability requirement
  - one external identity cannot link to multiple local accounts
  - token bodies and provider secrets are never persisted in this record
```
