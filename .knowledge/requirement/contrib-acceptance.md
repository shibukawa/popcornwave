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
  - public API has examples and unsupported-feature documentation
  - malformed and oversized input tests prove bounded failure
  - fuzz corpus runs under host Go and regression cases run under TinyGo
  - race or reentrancy behavior is documented for shared objects
  - api:cli-check discovers and compiles imported contrib packages
package_gates:
  requirement:contrib-auth-state:
    - concurrent Take returns a stored value exactly once
    - expiry and cancellation tests use an injected clock without sleeping
  requirement:contrib-auth-common:
    - malformed, non-canonical, duplicate, expired, and oversized vectors fail within configured limits
  requirement:contrib-cbor:
    - RFC 8949 valid, malformed, duplicate-key, limit, and deterministic-encoding vectors pass
    - COSE_Key fixtures preserve signed labels and byte strings
  requirement:contrib-passkey:
    - ES256 registration and authentication succeed against two independent authenticator implementations
    - negative vectors cover challenge, origin, RP ID hash, flags, user handle, algorithm, signature, counter, and backup state
  requirement:contrib-otel:
    - W3C propagation vectors pass
    - OTLP/HTTP JSON trace and log requests are accepted on standard endpoints by an OpenTelemetry Collector
    - retry exhaustion reports drops without writing telemetry to stdout
  requirement:contrib-reverse-proxy:
    - header, path, cancellation, streaming, and backend failure fixtures pass
  requirement:contrib-jwt:
    - RFC and adversarial algorithm-confusion vectors pass
  requirement:contrib-oauth:
    - Authorization Code plus S256 PKCE succeeds against two independent servers or conformance fixtures
    - negative vectors cover state, expiry, replay, callback errors, redirect URI, PKCE, and oversized token responses
  requirement:contrib-oidc:
    - Authorization Code plus PKCE succeeds against two independent providers or conformance fixtures
  requirement:contrib-database:
    - database/sql integration passes against each supported server version
  requirement:contrib-html-template:
    - invalid field paths, loop types, map keys, helpers, and JSON type graphs fail during generation with source positions
    - generated struct, slice, array, map, pointer, and JSON fixtures match golden output under host Go and TinyGo
    - HTML, attribute, URL, JavaScript, CSS, and JSON script XSS vectors remain escaped
  requirement:contrib-zstd:
    - default host Go build uses github.com/klauspost/compress/zstd and round-trips upstream output
    - decision:force-tinygo-logic host build and TinyGo build select the bounded encoder
    - raw, RLE, and single-match compressed streams decode with the reference zstd CLI
    - reported SHA-256 and strong ETag identify the emitted encoded representation
    - WithETag(false) emits the same representation without allocating or updating SHA-256
```
