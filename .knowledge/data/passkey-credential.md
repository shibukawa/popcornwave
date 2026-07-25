---
id: data:passkey-credential
type: data
title: Passkey Credential
---
A passkey credential associates a requirement:contrib-passkey credential record with one data:user-account.

```yaml
fields:
  account_id: data:user-account identifier
  credential_id: globally unique bytes
  user_handle: account passkey user handle
  public_key: normalized ES256 key
  algorithm: COSE algorithm
  sign_count: integer
  backup_eligible: bool
  backup_state: bool
  transports: bounded optional list
  created_at: timestamp
  last_used_at: optional timestamp
  label: optional user-visible string
rules:
  - multiple credentials per account are allowed
  - private key material never reaches the server
  - accepted counter and backup changes persist atomically
  - disabled or deleted credentials cannot authenticate
  - application owns persistence without a required repository type
```
