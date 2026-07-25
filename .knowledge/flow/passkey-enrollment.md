---
id: flow:passkey-enrollment
type: flow
title: Passkey Enrollment
---
An authenticated account adds a passkey after recent strong authentication.

```yaml
preconditions:
  - active data:user-account
  - recent OIDC, existing passkey, or approved administrator bootstrap
flow:
  - require CSRF-protected authenticated session
  - begin requirement:contrib-passkey registration with stable account user handle
  - exclude existing data:passkey-credential IDs
  - persist single-use ceremony state in requirement:contrib-auth-state
  - validate browser response and finish registration
  - atomically persist the new credential against the same account
  - rotate session when policy treats enrollment as an authentication-strength change
rules:
  - never enroll from an unauthenticated email or username claim
  - require user verification according to data:authentication-runtime-config
  - audit success and safe failure without logging credential material
```
