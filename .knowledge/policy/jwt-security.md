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
  - bound how long a refresh may keep failing before the cached set stops being trusted
staleness:
  rule: a cached key set has a maximum stale age beyond its cache TTL, after which a failing refresh fails verification instead of extending the cache
  reason: the reachability of the issuer is what a refresh proves; a key withdrawn from the published set is withdrawn whether or not this process can fetch the document that says so
  tension: refusing on an outage turns an unreachable issuer into an outage here, which is the same trade policy:token-revocation settles under unavailable, and is decided the same way rather than by defaulting to trust
  setting: KeySourceOptions.MaxStaleAge, measured past CacheTTL, defaulting to one hour and capped at 24
  no_unbounded_value: deliberate; an outage that outlasts the cap is an outage, and admitting tokens through it is not a decision a default should make quietly
  on_expiry: the cached set is dropped and verification fails, and a later successful fetch restores service without a restart
errors:
  - stable categories without token or key material
```
