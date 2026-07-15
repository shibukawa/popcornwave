---
id: decision:passkey-first-authentication
type: decision
title: Passkey-First Authentication Priority
---
Petitweb prioritizes phishing-resistant passkey support before password hashing convenience packages.

```yaml
status: accepted
priority:
  foundation: requirement:contrib-cbor
  authentication: requirement:contrib-passkey
deprioritized:
  - bcrypt password hashing helpers
  - scrypt password hashing helpers
  - Argon2id password hashing helpers
  - PBKDF2 password hashing helpers
constraints:
  - applications may use external password libraries independently
  - Petitweb never implements new password hashing primitives
  - future password packages must wrap reviewed implementations and pass the TinyGo matrix
rationale:
  - passkeys provide origin-bound public-key challenge-response authentication
  - CBOR is a reusable prerequisite for WebAuthn authenticator and COSE key data
  - password fallback expands credential storage, rate limiting, breach response, and recovery scope
```
