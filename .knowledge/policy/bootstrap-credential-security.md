---
id: policy:bootstrap-credential-security
type: policy
title: Bootstrap Credential Security
---
Issued temporary credentials are narrowly scoped to establishing or recovering a passkey.

```yaml
issuance:
  - generate sufficient entropy; do not accept a user-chosen temporary password
  - prefer delivering login ID and secret through separate trusted channels
  - bind the credential to one account, purpose, and expiry
verification:
  - rate-limit by credential, login ID, client, and deployment policy
  - return enumeration-safe failure responses
  - decrement attempts atomically
  - audit issuance, failure, consumption, expiry, and revocation without secrets
authorization:
  allowed:
    - begin and finish the bound passkey ceremony
    - inspect only the minimum account display data needed for enrollment
  forbidden:
    - application handlers
    - account administration
    - normal session creation before passkey persistence
completion:
  - atomically persist the passkey, activate the account, and consume the credential
  - replace the restricted enrollment session with a new normal session
  - invalidate abandoned restricted sessions after a short TTL
```
