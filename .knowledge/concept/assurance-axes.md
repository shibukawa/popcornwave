---
id: concept:assurance-axes
type: concept
title: Assurance Strength and Freshness
---
Session assurance is two independent axes, strength and freshness, and a named level is derived from them at check time rather than stored.

```yaml
axes:
  strength:
    question: what proof was made
    source: the data:session-assurance-state authentication method, ordered by deployment configuration
    changes: only at login, re-proof, or a login-method change
    provider_name: acr in OpenID Connect
  freshness:
    question: how long ago that proof was made
    source: the data:session-assurance-state authenticated_at timestamp
    changes: continuously, with no event
    provider_name: auth_time in OpenID Connect
requirement_shape:
  form: an optional minimum strength and an optional maximum age
  reason: RFC 9470 carries exactly this pair as acr_values and max_age, so a framework requirement maps to a provider request without translation
application_visibility:
  declared: freshness alone, per api:assurance-guard
  modeled_not_declared: strength, because the framework cannot order the methods it mounts
  reason:
    - oidc_only and passkey_only each produce one method, so an ordering has nothing to order
    - oidc_passkey produces two, and neither is stronger in general
    - an ordering is a deployment claim about its provider, not a property of the label
  effect: an application branches on two states, authenticated and recently proved; the rest of this ladder is configuration or framework-internal
derived_levels:
  anonymous: no session and no hint, so the login screen offers no account and no issuer and the user supplies both; a login problem, handled by policy:authenticated-path-protection
  identified: no session authority, only the retained hint of policy:session-downgrade; a login-screen presentation, with no handler branch
  active: a valid session satisfying no freshness bound
  recent: a valid session satisfying the age one operation asked for
  per_operation: the recent level with a zero window, which entering an area and acting destructively inside it distinguish by window rather than by kind
level_boundaries:
  anonymous_to_identified: a completed login writes the hint
  identified_to_anonymous: either hint bound of policy:session-downgrade is exceeded, or the user clears it
  active_to_identified: either session bound of policy:session-security is exceeded, or the user logs out, and only where a hint is configured
  active_to_anonymous: the same event where no hint is configured, which is the default, so identified is a level a deployment opts into rather than one it passes through
  recent_to_active: elapsed time alone, which policy:reauthentication is the only way back from
  note: the session bounds and the hint bounds are independent pairs, so a browser that keeps a hint is normally identified for far longer than it is active
storage:
  stored: strength and authenticated_at
  not_stored: the level, because recent is relative to a requirement rather than absolute
  consequence: two handlers on one request may disagree on whether the session is recent, which is the intended behavior
decay:
  - freshness decays with time alone and is regained only through policy:reauthentication
  - strength never decays; it changes only through a new proof
  - the absolute and idle expiry of policy:session-security stay independent bounds and are not assurance
boundary:
  - assurance states how well the subject is proved, never what the subject may do
  - a stronger method does not imply a fresher one, and a fresher proof does not imply a stronger one
```
