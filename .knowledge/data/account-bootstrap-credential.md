---
id: data:account-bootstrap-credential
type: data
title: Account Bootstrap Credential
---
An issued login ID and temporary secret authorize one bounded passkey enrollment, not reusable password login.

```yaml
fields:
  account_id: provisional data:user-account identifier
  login_id: administrator-issued public identifier
  secret_digest: digest or keyed digest of a generated high-entropy secret
  purpose: initial_passkey or recovery_passkey
  issued_at: timestamp
  expires_at: timestamp
  attempts_remaining: bounded integer
  consumed_at: nullable timestamp
lifecycle:
  - generate an unpredictable single-use secret
  - reveal the raw secret only at issuance
  - verify with constant-time comparison
  - atomically consume after successful passkey persistence
  - revoke on expiry, exhaustion, replacement, or administrator action
rules:
  - UI may call the secret a temporary password
  - protocol semantics remain a single-use bootstrap credential
  - never store or log the raw secret
  - never grant a normal authenticated application session by itself
  - application owns persistence and identifier format
```
