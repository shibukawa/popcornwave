---
id: data:session-assurance-state
type: data
title: Session Assurance State
---
Assurance state adds a provider-verified proof time to the plugin's own session payload, leaving the generic session package unchanged, and adds the `[auth]` keys that bound how stale a proof may be.

```yaml
ownership:
  principle: data:session-record owns when and how strongly, and plugin/auth owns what proved it and whether that is enough
  session_package: unchanged; authenticated_at and authentication_method already exist there and are already protocol-neutral
  plugin_auth: owns every field and key below
  why_not_the_record:
    - a provider proof time is an OpenID Connect concept, and passkey_only has no provider at all
    - adding it to the generic record would put a permanently empty field in every deployment that uses no provider
    - the plugin payload already carries issuer, subject, key claim, and key, so this belongs beside them
    - api:session-manager stores the payload as opaque typed data and never learns what an issuer is
existing_fields:
  authenticated_at: time of the most recent authentication-strength change, on data:session-record and already on the session view
  authentication_method: on data:session-record as an unordered label such as oidc or passkey
added_fields:
  location: the plugin/auth session payload, not data:session-record
  provider_authenticated_at: verified auth_time of the identity provider when the proof came from OIDC, supplied by requirement:contrib-oidc
  reauth_count: completed re-proofs on this session, for audit only
strength:
  today: the existing unordered authentication_method label, recorded and audited but never compared
  deferred: an ordered label set and the auth.assurance.strengths key, for the reason api:assurance-guard gives
  effect: nothing in the request view ranks one method above another, so no handler can accidentally depend on an ordering the framework invented
provider_time:
  reason: a provider may satisfy a max_age request from its own single sign-on session, so the callback time is not the proof time
  rule: freshness is measured from provider_authenticated_at when present, otherwise from authenticated_at
  absent: a provider returning no auth_time cannot satisfy a freshness requirement through OIDC re-proof
config_fields:
  binding: every key is under [auth], because none of them means anything to a session that carries no identity
  auth.recent_auth_max_age: the existing duration, default 5m, which becomes the default maximum age of a guard naming none
  auth.assurance.policy.<name>.max_age: a named window an api:assurance-guard Requirement resolves, so user experience is deployment-tunable
  auth.assurance.hint: the keys policy:session-downgrade defines, which are the hint's own absolute and idle bounds and not the session's
absolute_expiry_on_reproof:
  behavior: re-proof resets it, because api:session-manager Rotate revokes and creates, and creation stamps a fresh CreatedAt and ExpiresAt
  aligned_with: the reauthentication rule of policy:session-security and the NIST rule that a successful reauthentication resets both timeouts
  not_configurable: preserving the original absolute expiry across a rotation would need a new session-package capability, and no requirement asks for one
  revisit_when: a deployment states that its absolute expiry is a theft bound it needs held across re-proof
  side_effect: CreatedAt is restamped, so a session keeps no memory of when it first began; nothing reads that today
validation:
  - auth.recent_auth_max_age stays positive whenever any guard or enrollment path is reachable
  - a guard may name a max age of zero, which means prove again for this operation and is not the same as naming none
  - a Requirement naming an undefined auth.assurance.policy entry fails startup rather than at request time
  - every auth.assurance.policy entry declares max_age, and a zero value is valid and carries the zero_semantics of api:assurance-guard
  - data:authentication-runtime-config mode_validation refuses an assurance key the selected mode cannot honor
rules:
  - the request view exposes the method label and effective freshness, never a token, an ID Token body, or a raw claim
  - re-proof updates authenticated_at, provider_authenticated_at, authentication_method, and reauth_count in one rotation
  - re-proof resets idle and absolute expiry together, per absolute_expiry_on_reproof above
  - the method label changes only through a new proof, never through elapsed time
  - a stored record predating these fields reads as having no provider proof time rather than failing, which the data:session-record version field already governs
```
