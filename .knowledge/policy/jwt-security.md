---
id: policy:jwt-security
type: policy
title: JWT Security Policy
---
JWT acceptance is fail-closed and binds algorithm, key, token type, issuer, audience, and time validation to caller policy.

```yaml
required:
  - exact algorithm allowlist supplied by verifier configuration
  - reject alg none and unsupported critical headers
  - never select verification behavior solely from attacker-controlled alg
  - enforce compact token and decoded segment size limits
  - reject malformed Base64url and trailing data
  - reject duplicate security-relevant JSON member names
  - constant-time MAC comparison
  - require exp by default
  - validate nbf, exp, issuer, audience, and token type
  - use injectable clock and bounded leeway
  - reject invalid numeric date precision or overflow
jwks:
  - select by kid plus configured algorithm and key use
  - reject ambiguous matches
  - cap key count and document size
errors:
  - stable categories without token or key material
```
