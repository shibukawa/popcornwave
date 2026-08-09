---
id: policy:public-asset-negotiation
type: policy
title: Public Asset Negotiation
---
Public assets select one precompressed representation with standards-compliant HTTP content negotiation, from the codings policy:public-asset-precompression produced.

```yaml
scope: production and non-pw-dev execution
codings:
  offered: br, zstd, gzip
  order: br, then zstd, then gzip, then identity
  why_that_order: pure ratio, because every representation was encoded at build time and none of them costs request latency, which is what separates this ordering from the throughput-weighted one of policy:response-content-encoding
selection:
  - parse Accept-Encoding tokens, wildcards, and q-values
  - absent or empty Accept-Encoding selects identity
  - a coding is a candidate when it is explicitly acceptable with q greater than zero, or reached through a wildcard with q greater than zero, and a sidecar for it exists
  - among candidates choose by the order above, not by client q-value, matching policy:public-asset-media-negotiation
  - prefer the original identity representation when no coding is a candidate
  - return 406 when every available representation is explicitly unacceptable
encoded_response:
  Content-Encoding: the selected coding token
  Content-Type: media type of the original path
  Content-Length: encoded byte length
common_response:
  Vary: Accept-Encoding
  methods: GET and HEAD
rules:
  - api:public-asset-middleware development implementation bypasses this policy
  - HEAD returns the selected representation headers without a body
  - precompressed selection is independent from data:compression-runtime-config dynamic compression enablement
  - representation-specific validators must not reuse one strong ETag across codings; each representation carries the tag of its own bytes, per data:public-asset-manifest
  - runtime response-compression middleware must not recompress this handler response, per the scope exclusion in policy:response-content-encoding
  - nothing is encoded or decoded at request time, whatever the client sent
references:
  - https://www.rfc-editor.org/rfc/rfc9110.html#name-accept-encoding
  - https://www.rfc-editor.org/rfc/rfc9659
```
