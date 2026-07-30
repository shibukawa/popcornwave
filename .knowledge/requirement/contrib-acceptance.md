---
id: requirement:contrib-acceptance
type: requirement
title: Contrib Acceptance Criteria
---
A contrib package is releasable only when its declared subset is interoperable, bounded under hostile input, and verified on its supported TinyGo matrix.

```yaml
common:
  - policy:contrib-compatibility checks pass
  - host Go and TinyGo produce equivalent fixture results
  - OAuth/OIDC compatibility is demonstrated with at least two independent protocol-shaped fixtures
  - live provider interoperability is tracked separately and is not claimed without provider credentials
  - public API has examples and unsupported-feature documentation
  - malformed and oversized input tests prove bounded failure
  - fuzz corpus runs under host Go and regression cases run under TinyGo
  - race or reentrancy behavior is documented for shared objects
  - package target matrices compile supported imported packages
package_gates:
  requirement:contrib-auth-state:
    - concurrent Take returns a stored value exactly once
    - every adapter preserves single-use, expiry, cancellation, and stable error semantics
  requirement:contrib-auth-state-memory:
    - expiry and capacity tests use an injected clock without sleeping
    - concurrent Put and Take pass under the race detector
    - hard entry and key limits reject oversized configuration
  requirement:contrib-auth-state-redis:
    - Redis and Valkey each pass duplicate Put, expiry, concurrent Take, malformed record, codec failure, cancellation, and reconnect fixtures
    - every external network fixture follows policy:outbound-transport-security
  requirement:contrib-auth-state-sqlite:
    - every requirement:contrib-sqlite backend passes duplicate Put, expired replacement, concurrent Take, malformed row, codec failure, cancellation, reopen, and bounded Prune fixtures
    - schema initialization is idempotent and rejects incompatible existing tables
  requirement:contrib-auth-common:
    - malformed, non-canonical, duplicate, expired, and oversized vectors fail within configured limits
  requirement:contrib-cbor:
    - RFC 8949 valid, malformed, duplicate-key, limit, and deterministic-encoding vectors pass
    - COSE_Key fixtures preserve signed labels and byte strings
  requirement:contrib-passkey:
    - ES256 registration and authentication succeed against two independent authenticator fixtures or implementations
    - negative vectors cover challenge, origin, RP ID hash, flags, user handle, algorithm, signature, counter, and backup state
  requirement:contrib-passkey-test:
    - a registration and an authentication built from the emitted options JSON alone complete against requirement:contrib-passkey
    - every declared fault is rejected with its specific requirement:contrib-passkey error and none is accepted
    - a seeded run reproduces identical credential IDs and keys; signatures still vary, because ECDSA is randomized exactly as it is on a real authenticator
    - two credentials registered for one account are independently selectable through allowCredentials
    - discoverable and non-discoverable registrations differ only in the returned user handle
    - it counts as at most one of the two independent authenticator implementations, per decision:passkey-test-authenticator
    - the TinyGo matrix is not required, per decision:host-tools-target-runtime
  requirement:contrib-otel:
    - W3C propagation vectors pass
    - OTLP/HTTP JSON trace and log requests are accepted on standard endpoints by an OpenTelemetry Collector
    - retry exhaustion reports drops without writing telemetry to stdout
  requirement:contrib-redis-valkey:
    - the pinned go-redis client compiles with the supported TinyGo version and passes required commands against Redis and Valkey
    - decision:local-tls-proxy-boundary passes certificate, hostname, unavailable upstream, and credential-redaction fixtures
  requirement:contrib-jwt:
    - RFC and adversarial algorithm-confusion vectors pass
  requirement:contrib-oauth:
    - Authorization Code plus S256 PKCE succeeds against two independent servers or conformance fixtures
    - negative vectors cover state, expiry, replay, callback errors, redirect URI, PKCE, and oversized token responses
  requirement:contrib-oidc:
    - Authorization Code plus PKCE succeeds against two independent providers or conformance fixtures
    - requirement:contrib-devidp counts as at most one of those two, because it is developed in this repository
  requirement:contrib-devidp:
    - an independent OIDC client library validates discovery, JWKS, ID Token, and UserInfo output
    - requirement:contrib-oidc completes Authorization Code plus S256 PKCE against it under host Go
    - negative vectors cover unknown client, unregistered redirect URI, missing or mismatched PKCE verifier, replayed code, expired code, wrong client secret, unknown subject, and unconfigured scope
    - roster loading rejects unknown keys, reserved claim overrides, empty client secrets, and duplicate subjects
    - policy:devidp-safety refusals are proven for prod environment, non-loopback bind without opt-in, and application build import
    - automatic login and ui:devidp-login selection issue identical token claims
    - RP-initiated logout revokes the subject access tokens and rejects a forged id_token_hint or an unregistered post_logout_redirect_uri
    - an api:cli-dev run with no client, issuer, or secret in any project file completes flow:oidc-account-login from injected environment values alone
    - a reserved-port run and a pinned-port run both produce discovery URLs that the relying party accepts
    - ephemeral client redirect handling accepts loopback callbacks on any port and rejects every non-loopback host
    - injected secrets never appear in api:cli-dev output or configuration provenance logs
    - the TinyGo matrix is not required, per decision:devidp-scope-reduction
  requirement:contrib-html-template:
    - invalid field paths, loop types, map keys, helpers, and JSON type graphs fail during generation with source positions
    - generated struct, slice, array, map, pointer, and JSON fixtures match golden output under host Go and TinyGo
    - HTML, attribute, URL, JavaScript, CSS, and JSON script XSS vectors remain escaped
```
