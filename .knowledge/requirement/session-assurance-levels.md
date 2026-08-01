---
id: requirement:session-assurance-levels
type: requirement
title: Graded Session Assurance
---
A session carries graded assurance rather than one valid-or-not flag, so an application gates a sensitive operation on how recently and how strongly the identity was proved, and an unmet requirement leads to re-proof instead of a dead end.

```yaml
audience: actor:application-developer
existing_partial:
  fields: data:session-record already stores authenticated_at and authentication_method, and the session view already exposes both
  consumer: api:passkey-endpoints register/begin refuses a session older than auth.recent_auth_max_age
  limits:
    - the check is framework-internal, so no application handler can express it
    - one global duration serves every operation
    - the refusal is HTTP 403 with no path back to freshness
    - the provider half is now in place: requirement:contrib-oidc sends max_age and prompt and returns a verified auth_time, so an OIDC proof can be asked to be fresh and the answer can be checked
model: concept:assurance-axes
application_states:
  count: two
  authenticated: policy:authenticated-path-protection already provides it, declared by path in configuration
  recently_proved: the whole of the new application-facing surface, declared per operation
  principle: a level an application never branches on is configuration or presentation, not API; only freshness earns a handler call
required:
  - freshness is readable request state, per data:session-assurance-state
  - a route or handler declares the recency it needs without naming a provider, per api:assurance-guard
  - an unmet requirement produces re-proof that resumes the original operation, per flow:step-up-reauthentication
  - re-proof refreshes the existing session and never changes the account, per policy:reauthentication
  - a logout chooses what it does to the provider session, per policy:provider-session-scope, and shares the step-up reconfirmation mechanism
  - an ended session may leave a non-authoritative identity hint, per policy:session-downgrade
  - assurance changes are auditable without tokens, claims, credential IDs, or cookie values
acceptance:
  - a guarded handler on a stale session reaches re-proof and returns to the same operation
  - a requirement is declared where the route is registered, so reading the mux shows which routes are sensitive
  - a guarded handler on fresh proof runs with no provider round trip
  - re-proof completed by a different account is refused instead of swapping the session subject
  - the existing api:passkey-endpoints enrollment check is expressed through this guard and keeps its current default
  - the same handler compiles unchanged under oidc_only, oidc_passkey, and passkey_only
  - a backend that cannot revoke states what it cannot enforce instead of appearing to enforce it
non_goals:
  - authorization by role, tenant, ownership, or permission, which stays application policy
  - a handler-declared minimum authentication strength, deferred for the reason api:assurance-guard gives
  - risk scoring from device, geography, or behavior signals
  - intake of external revocation signals such as continuous access evaluation
  - contact-channel verification, per decision:assurance-scope-oidc-only
standard:
  reauthentication: https://pages.nist.gov/800-63-4/sp800-63b/session/
  step_up_challenge: https://www.rfc-editor.org/rfc/rfc9470.html
```
