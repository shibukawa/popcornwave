---
id: policy:public-asset-negotiation
type: policy
title: Public Asset Negotiation
---
Public assets select a precompressed Zstandard representation with standards-compliant HTTP content negotiation.

```yaml
scope: production and non-pw-dev execution
selection:
  - parse Accept-Encoding tokens, wildcards, and q-values
  - absent or empty Accept-Encoding selects identity
  - choose zstd only when zstd or wildcard is explicitly acceptable with q greater than zero and a matching sidecar exists
  - prefer the original identity representation otherwise
  - return 406 when every available representation is explicitly unacceptable
zstd_response:
  Content-Encoding: zstd
  Content-Type: media type of the original path
  Content-Length: encoded byte length
common_response:
  Vary: Accept-Encoding
  methods: GET and HEAD
rules:
  - api:public-asset-middleware development implementation bypasses this policy
  - HEAD returns the selected representation headers without a body
  - precompressed selection is independent from data:compression-runtime-config dynamic compression enablement
  - representation-specific validators must not reuse one strong ETag across identity and zstd bytes
  - runtime response-compression middleware must not recompress this handler response
references:
  - https://www.rfc-editor.org/rfc/rfc9110.html#name-accept-encoding
  - https://www.rfc-editor.org/rfc/rfc9659
```
