---
id: data:authentication-runtime-config
type: data
title: Authentication Runtime Config
---
The `[auth]` binding selects OIDC and passkey bootstrap, login, linking, registration, and recovery policy through shared dotted prefixes.

```yaml
registration: the framework registers the implemented subset; the optional authentication package extends it
implemented:
  keys:
    - auth.enabled
    - auth.mode
    - auth.login_path
    - auth.callback_path
    - auth.logout_path
    - auth.post_login_redirect
    - auth.post_logout_redirect
    - auth.oidc.issuer
    - auth.oidc.client_id
    - auth.oidc.client_secret
    - auth.oidc.redirect_url
    - auth.oidc.scopes
    - auth.oidc.provider_logout
  endpoints: api:authentication-endpoints mounts the three paths
  defaults:
    login_path: /auth/login
    callback_path: /auth/callback
    logout_path: /auth/logout
    post_login_redirect: /
    post_logout_redirect: /
    provider_logout: true
  validation:
    - an unknown mode fails startup
    - an OIDC mode with an empty issuer, client id, or client secret fails startup naming every missing key and its environment variable
    - the failure text points at api:cli-dev, the environment, and the resolved config file
    - issuer and redirect_url must be absolute URLs
    - every endpoint and redirect path must be a same-origin absolute path, so a configuration mistake cannot become an open redirect
    - the three endpoint paths must differ
  reason: an incomplete provider configuration is a startup defect, not a first-login surprise
  scaffold: api:cli-init writes this section for the selected authentication mode
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
  oidc.admission: existing, claim, or authenticated
  oidc.auto_provision: bool
  oidc.claim.path: JSON Pointer
  oidc.claim.values: string list
  oidc.claim.match: any or all
  passkey.rp_id: domain
  passkey.rp_name: string
  passkey.origins: URL list
  passkey.user_verification: required, preferred, or discouraged
  passkey.discoverable: required or preferred
development_injection:
  source: api:cli-dev starting requirement:contrib-devidp
  variables:
    - AUTH_OIDC_ISSUER
    - AUTH_OIDC_CLIENT_ID
    - AUTH_OIDC_CLIENT_SECRET
  precedence: data:loaded-configuration ranks environment above TOML, so no development TOML edit is required
  redirect_url:
    optional_when: data:runtime-environment is dev and the request host is loopback
    derivation: request scheme and host joined with callback_path
    otherwise: required and validated as an absolute URL
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
