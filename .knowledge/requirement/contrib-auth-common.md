---
id: requirement:contrib-auth-common
type: requirement
title: Authentication Security Primitives
---
contrib authentication packages share small bounded primitives without sharing protocol-specific signature policy.

```yaml
package: contrib/internal/authn
consumers:
  - requirement:contrib-jwt
  - requirement:contrib-passkey
  - requirement:contrib-oauth
  - requirement:contrib-oidc
required:
  - crypto/rand Base64url secret generation with explicit byte count
  - constant-time exact secret comparison
  - strict bounded Base64url decoding
  - injectable clock and expiry validation
  - PKCE verifier validation and S256 challenge derivation
  - bounded duplicate-aware JSON parsing
  - bounded HTTP response reading and redirect policy hooks
boundaries:
  - challenge, state, nonce, and verifier remain distinct protocol types
  - JWT algorithm and key selection remain in requirement:contrib-jwt
  - WebAuthn COSE key and ES256 verification remain in requirement:contrib-passkey
  - no generic algorithm selected from attacker-controlled input
```
