---
id: data:authentication-runtime-config
type: data
title: Authentication Runtime Config
---
The `[auth]` binding selects OIDC and passkey bootstrap, login, linking, registration, and recovery policy through shared dotted prefixes.

```yaml
registration: optional authentication package registers this binding when imported
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
  bootstrap.credential_ttl: duration
  bootstrap.session_ttl: duration
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
  passkey.rp_id: domain
  passkey.rp_name: string
  passkey.origins: URL list
  passkey.user_verification: required, preferred, or discouraged
  passkey.discoverable: required or preferred
implemented:
  package: popcornwave/plugin/auth registered through api:framework-extension
  mode: oidc_only
  endpoints: login_path begins authorization, callback_path completes it, logout_path revokes the local session
  correlation: the opaque transaction key rides a short-lived cookie scoped to callback_path
  discovery: deferred to the first login and not cached on failure
  storage: requires session backend rdb over sqlite, because correlation records use requirement:contrib-auth-state-sqlite
  tables: rule:framework-owned-tables migrations, verified at startup
deferred:
  - oidc_passkey and passkey_only modes, rejected during startup validation
  - registration, recovery, recent_auth_max_age, and bootstrap settings
  - policy:csrf-protection
  - RP-initiated logout at the provider
loopback_development:
  field: oidc.allow_loopback_http
  effect: permits an http issuer whose host is loopback
  restriction: development only, and paired with session cookie secure false
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
