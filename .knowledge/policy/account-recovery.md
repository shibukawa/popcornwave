---
id: policy:account-recovery
type: policy
title: Account Recovery Policy
---
Every deployment chooses an explicit recovery authority before enabling account registration.

```yaml
oidc_passkey:
  preferred: trusted OIDC reauthentication followed by passkey replacement
  alternative: administrator-reviewed recovery
passkey_only:
  allowed:
    - another already-enrolled passkey
    - administrator-reviewed recovery that issues a new data:account-bootstrap-credential
    - application-provided verified recovery mechanism
  forbidden_default: possession of email address alone
bootstrap_recovery:
  - bind the new credential to recovery_passkey purpose
  - grant only a restricted passkey enrollment session
  - require successful new passkey persistence before normal access
  - revoke or review existing credentials and sessions according to deployment policy
rules:
  - require recent strong proof before removing the last usable credential
  - revoke or rotate sessions after recovery
  - notify through an independently verified channel when available
  - rate-limit and audit recovery attempts
  - do not claim recoverability when no recovery authority is configured
  - apply policy:bootstrap-credential-security to every issued recovery secret
```
