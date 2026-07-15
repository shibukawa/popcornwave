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
  requirement:contrib-httpmux:
    - Go standard-library routing fixtures produce equivalent handler, pattern, status, Allow, redirect, and PathValue results
    - registration panic fixtures cover invalid, duplicate, and conflicting patterns
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
  requirement:contrib-oidc:
    - Authorization Code plus PKCE succeeds against two independent providers or conformance fixtures
  requirement:contrib-database:
    - database/sql integration passes against each supported server version
  requirement:contrib-html-template:
    - supported syntax matches host html/template output and XSS vectors remain escaped
  requirement:contrib-zstd:
    - decode and encode streams interoperate with the reference zstd CLI
```
