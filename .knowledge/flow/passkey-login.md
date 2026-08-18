---
id: flow:passkey-login
type: flow
title: Passkey Login
---
Passkey login resolves a verified credential to its local account and creates a normal Popcorn Web session.

```yaml
flow:
  - begin requirement:contrib-passkey authentication in discoverable or account-selected mode
  - persist single-use ceremony state in requirement:contrib-auth-state
  - resolve data:passkey-credential by credential ID
  - verify assertion, origin, RP ID, user presence, and configured user verification
  - reject inactive account or credential
  - evaluate counter risk and atomically persist accepted counter and backup state
  - create or rotate api:session-manager session
  - record data:request-authentication method as passkey
rules:
  - credential possession alone does not bypass account state or authorization checks
  - repeated or superseded ceremony state fails closed
```
