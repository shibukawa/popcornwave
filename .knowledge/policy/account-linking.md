---
id: policy:account-linking
type: policy
title: Account Linking Policy
---
Link external identities and passkeys only through authenticated, conflict-free proof bound to one local account.

```yaml
rules:
  - external identity key is canonical issuer plus verified subject
  - never auto-link accounts solely because email strings match
  - linking to an existing account requires a current authenticated session and recent strong authentication
  - reject a link already owned by another account
  - require explicit user confirmation before adding or removing a login method
  - rotate session after login-method changes
  - retain an audit event without tokens, cookie values, credential IDs, or raw claims
optional_email_assist:
  - may suggest a candidate only when issuer is trusted and email_verified is true
  - still requires authenticated confirmation and conflict checks
```
