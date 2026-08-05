---
id: requirement:auth-security-remediation
type: requirement
title: Authentication Security Remediation
---
The eight gaps a security review found in the authentication surface at HEAD 8c9601c, all closed; each invariant now lives in the concept that owns it, and this records what was decided and what remains bounded rather than absolute.

```yaml
review:
  date: 2026-08-05
  head: 8c9601c
  scope: plugin/auth, contrib/passkey, contrib/jwt, contrib/oidc, session, and the DynamoDB and relational authentication stores
  verified: every finding was read against the tree rather than carried over from the report
  found_no_fault_in: session encryption, the OAuth and OIDC exchanges, JWT validation, passkey signature verification, and CSRF token generation
status: implemented
closed:
  passkey_enrollment_binding:
    concept: policy:passkey-security
    shipped: an opaque binding carried in the ceremony state and required as an argument to FinishRegistration
  account_revocation:
    concept: policy:session-security
    shipped: the account behind a live session is re-read, bounded to once per 30 seconds, with auth.ForgetAccount for immediacy
  login_slot_placement:
    concept: policy:session-security
    shipped: a startup warning outside dev plus the PW0506 readiness check, rather than a refusal
  tinygo_request_timeouts:
    concept: rule:tinygo-runtime-compatibility
    shipped: a deadline-enforcing transport applied by the OIDC and JWT clients under the tinygo tag
  counter_monotonicity:
    concept: api:auth-credential-store
    shipped: a conditional relational update matching the DynamoDB store, with the rule stated on the interface
  jwks_staleness:
    concept: policy:jwt-security
    shipped: MaxStaleAge, defaulting to an hour and capped at 24, with no unbounded value
  revocation_cache_bounds:
    concept: policy:token-revocation
    shipped: a 4096-entry cap with expiry-driven eviction
  origin_check_consolidation:
    concept: policy:csrf-protection
    shipped: internal/requestorigin, shared by the CSRF middleware and the authentication endpoints
changed_from_the_plan:
  login_slot_placement:
    planned: register the slot session.ServerOnly and let the session package refuse the cookie backend
    shipped: a startup warning outside dev, silent in dev, and PW0506 in pw doctor
    why: the slot is registered from an init that cannot see whether auth is enabled, so ServerOnly would take the cookie backend away from a deployment that merely links this package; the slot also stores nothing before a login, so the placement bought nothing a check does not
    why_not_a_refusal: a login on the cookie backend is the correct pairing in development, where the backend exists so that a login needs no infrastructure; refusing would remove it from the case it was built for, and the deployment is the party that knows whether it can live without ending a session on demand
  account_revocation:
    planned: an account-scoped epoch stamped into the session and compared per request
    shipped: a bounded re-read of the account
    why: an epoch still has to be fetched from somewhere to be compared against, so it adds a field without removing the read; the application already publishes one way to ask about an account
sharpened_during_verification:
  origin_check: the middleware implementation was already correct, reconstructing a whole origin and refusing a null one, so the finding was a second weaker copy guarding the authentication endpoints rather than a missing check
  passkey_binding: the stored user handle also stays the one minted at begin, so an affected account would have held two handles
residual:
  documented_at: the Known limits section of the security guide, in both locales
  items:
    - a suspension reaches a live session within 30 seconds, and ForgetAccount is process-local
    - a TinyGo round trip cannot be cancelled, so a hung provider costs a goroutine and a connection rather than a stalled handler
    - cached JWKS keys stop verifying an hour past their TTL during an issuer outage
    - the cookie session backend still cannot revoke; outside dev that is a startup warning and PW0506, not a refusal, so a deployment can still choose it knowingly
verification:
  passed: go build, go vet under both build configurations, go test over the whole tree, and the documentation site build
  tinygo: tinygo test of contrib/internal/authn on 0.41.1 darwin/arm64, covering the deadline transport
  added_tests: internal/requestorigin, contrib/jwt RemoteKeySet which had none, the passkey binding, the account gate, the conditional counter, the revocation cache bounds, and the cookie-backend refusal
  unchanged_from_the_review: the force_tinygo_logic run over the whole tree still fails in zstd response handling and a SQLite read-only transaction, neither in the authentication surface
  caveat: system:tinygodriver vendors forks of its database drivers, so a vulnerability database may not recognize the module names; govulncheck passing is weaker evidence there than elsewhere
non_goals:
  - a finding type in the catalog; each invariant belongs to the concept it constrains, and the defect record is deleted with the fix
  - configuration keys for the two new bounds, which are constants until a deployment asks for them
```
