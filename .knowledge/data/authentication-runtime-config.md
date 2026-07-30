---
id: data:authentication-runtime-config
type: data
title: Authentication Runtime Config
---
The `[auth]` binding selects OIDC and passkey bootstrap, login, linking, registration, and recovery policy through shared dotted prefixes.

```yaml
registration: popcornwave/plugin/auth registers this binding when imported
fields:
  enabled: bool
  mode: oidc_passkey, oidc_only, or passkey_only
  login_path: path
  callback_path: path
  protection.include: path pattern list
  protection.exclude: path pattern list
  protection.unauthenticated: redirect or unauthorized
  registration.policy: disabled, oidc, invite, administrator, or open
  recovery.policy: oidc, administrator, or application
  recent_auth_max_age: duration
  bootstrap.issue_ttl: duration an issued secret stays redeemable, measured from issuance
  bootstrap.enrollment_ttl: duration the enrollment stays open, measured from redemption
  bootstrap.max_attempts: positive integer
  oidc.issuer: URL
  oidc.client_id: string
  oidc.client_secret: secret
  oidc.redirect_url: URL
  oidc.scopes: string list
  oidc.identity_claim: claim name
  oidc.admission: existing, claim, registered, or authenticated
  oidc.auto_provision: bool
  oidc.claim.path: JSON Pointer
  oidc.claim.values: string list
  oidc.claim.match: any or all
  oidc.allow_loopback_http: bool
  oidc.registered_claims: string list
  logout_path: path
  post_login_path: path
  passkey.path: base path of api:passkey-endpoints
  passkey.rp_id: domain
  passkey.rp_name: string
  passkey.origins: URL list
  passkey.user_verification: required, preferred, or discouraged
  passkey.discoverable: required or preferred
mode_validation:
  principle: validate and read only the fields the selected mode uses, and refuse a field the mode cannot honor, because a silently ignored provider setting reads as configured security
  oidc_only:
    required: the oidc issuer, client_id, client_secret, and redirect_url set today
    refused: every passkey field, because no ceremony endpoint exists to use it
  oidc_passkey:
    required: the oidc_only set, plus passkey.rp_id and passkey.origins
    required_policy: recovery.policy, because two login methods make credential loss recoverable in more than one way
    defaulted: registration.policy oidc, because the OIDC login is the account bootstrap
  passkey_only:
    required: passkey.rp_id and passkey.origins
    required_policy: registration.policy and recovery.policy, both explicit, because no identity provider can stand in for either
    required_bootstrap: bootstrap.issue_ttl, bootstrap.enrollment_ttl, and bootstrap.max_attempts when registration.policy is administrator or invite
    bootstrap_naming: issue_ttl rather than credential_ttl, because the two durations bound consecutive phases and the name should say which; a leading noun also kept it out of the secret-redaction match
    refused: every oidc field, so a leftover AUTH_OIDC_ISSUER cannot suggest a provider is in the loop
  shared:
    - passkey.rp_id must be a registrable domain or localhost, never an IP literal, because an IP cannot be an RP ID
    - every passkey.origins entry must be https, or loopback http under the same allowance oidc.allow_loopback_http already carries
    - every passkey.origins entry must have passkey.rp_id as its registrable suffix
    - recent_auth_max_age must be positive whenever enrollment is reachable
implemented:
  package: popcornwave/plugin/auth registered through api:framework-extension
  mode: oidc_only, oidc_passkey, and passkey_only
  endpoints: login_path begins authorization, callback_path completes it, logout_path revokes the local session and ends the provider session
  logout_method: POST only, same-origin checked, because a logout reachable by link or prefetch is a denial-of-service surface
  correlation: the opaque transaction key rides a short-lived cookie scoped to callback_path
  discovery: deferred to the first login and not cached on failure
  storage: requires session backend rdb over sqlite, because correlation records use requirement:contrib-auth-state-sqlite
  tables: rule:framework-owned-tables migrations, verified at startup
binding_implemented:
  fields: registration, recovery, recent_auth_max_age, bootstrap, and the whole passkey prefix are bound and validated
  validation: mode_validation above is enforced, so a passkey mode is refused for a bad relying-party registration before anything serves
  tables: popcornwave_passkey_credential and popcornwave_auth_bootstrap exist under rule:framework-owned-tables
  modes: all three serve; there is no remaining implementation gate
  reason: the rules outlive the implementation status, so they were written and tested before the endpoints that needed them
planned:
  testing: decision:test-authentication-seams
deferred:
  - policy:csrf-protection, until then api:passkey-endpoints relies on same-origin and a required JSON content type
loopback_development:
  field: oidc.allow_loopback_http
  effect: permits an http issuer whose host is loopback
  restriction: development only, and paired with session cookie secure false
development_injection:
  source: api:cli-dev starting requirement:contrib-devidp
  binding: the injected names are the ones plugin/auth already reads, so no development TOML edit is required
  variables:
    - AUTH_OIDC_ISSUER
    - AUTH_OIDC_CLIENT_ID
    - AUTH_OIDC_CLIENT_SECRET
  precedence: data:loaded-configuration ranks environment above TOML
  issuer_scheme: oidc.allow_loopback_http is required, because the development issuer is loopback http
rules:
  - decision:authentication-bootstrap-strategy defines mode behavior
  - policy:authenticated-path-protection defines pattern matching and middleware behavior
  - redirect response targets the local login_path
  - unauthorized response returns HTTP 401 without navigation
  - validate only provider and policy fields used by the selected mode
  - existing admission requires auto_provision false
  - claim admission requires a non-empty path and values
  - authenticated admission with auto_provision permits every verified issuer subject to create an account
  - require data:session-runtime-config when provider flow needs login continuity
  - redact provider secrets
  - derive oidc.redirect_url from the request only under the development_injection conditions, never from a forwarded or non-loopback host
  - verified request results become data:request-authentication
  - passkey-only mode requires explicit registration and policy:account-recovery settings
  - administrator registration and recovery require bounded bootstrap settings
flows:
  - flow:oidc-account-login
  - flow:passkey-enrollment
  - flow:passkey-login
  - flow:passkey-only-registration
security:
  - policy:account-linking
  - policy:account-recovery
  - policy:bootstrap-credential-security
  - policy:oidc-admission
  - policy:authenticated-path-protection
```
