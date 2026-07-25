---
id: data:user-account
type: data
title: Local User Account
---
A local account is the stable application identity shared by OIDC identities, passkey credentials, authorization, and sessions.

```yaml
required:
  id: stable opaque application identifier
  passkey_user_handle: stable random opaque bytes
  state: provisional, active, suspended, or deleted
  created_at: timestamp
optional:
  display_name: string
  verified_contact: application-defined
relations:
  external_identities: data:external-identity list
  passkeys: data:passkey-credential list
rules:
  - never use email address as the primary account key
  - passkey user handle is not derived from email, username, or database sequence
  - activation follows a completed trusted bootstrap flow
  - suspended and deleted accounts cannot create sessions
  - storage schema and generated SQL functions remain application-owned
```
