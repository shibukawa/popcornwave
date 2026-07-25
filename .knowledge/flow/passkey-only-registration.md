---
id: flow:passkey-only-registration
type: flow
title: Passkey-Only Registration
---
Passkey-only deployments may issue a login ID and temporary secret that open only a restricted first-passkey enrollment.

```yaml
policies:
  disabled: no self-registration
  invite: require a single-use bounded invitation
  administrator: provision account and data:account-bootstrap-credential out of band
  open: explicit opt-in with abuse controls
flow:
  - allocate provisional data:user-account with random stable user handle
  - issue login ID and one-time temporary secret under policy:bootstrap-credential-security
  - verify the ID and secret without revealing account existence
  - create a short-lived enrollment-only session; do not create a normal application session
  - begin requirement:contrib-passkey registration
  - persist single-use ceremony state in requirement:contrib-auth-state
  - finish registration and atomically save data:passkey-credential, activate the account, and consume the bootstrap credential
  - replace the enrollment-only session with an api:session-manager normal session
failure:
  - leave no active account without a credential
  - expire or clean abandoned provisional accounts
  - revoke expired, exhausted, replaced, or consumed bootstrap credentials
rules:
  - the temporary secret is not a reusable password authentication method
  - email input is unverified unless a separate verification flow proves it
  - open registration requires rate limits and enumeration-safe responses
  - policy:account-recovery must be selected before enabling passkey-only registration
```
