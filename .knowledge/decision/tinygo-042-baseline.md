---
id: decision:tinygo-042-baseline
type: decision
title: TinyGo 0.42 Baseline
---
Petitweb targets TinyGo 0.42 or later and does not carry compatibility packages solely for older TinyGo standard libraries.

```yaml
status: accepted
minimum_tinygo: "0.42"
stdlib_http:
  router: net/http.ServeMux
  required_semantics: "Go 1.22+ method and path patterns"
  implementation_evidence: "tinygo-org/net HTTP sources derived from Go 1.26.2"
verification:
  - api:cli-check rejects older TinyGo versions
  - examples/httpserver builds with TinyGo 0.42
  - ServeMux method, wildcard, PathValue, 405, and Allow fixtures pass
source: https://github.com/tinygo-org/net/tree/d5da3ddeef797c23fba7fd4a1f47306216e63a4e/http
```
