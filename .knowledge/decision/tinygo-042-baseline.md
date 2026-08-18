---
id: decision:tinygo-042-baseline
type: decision
title: TinyGo 0.42 Baseline
---
Popcorn Web targets TinyGo 0.42 or later and obtains reusable networking compatibility from system:tinygodriver instead of carrying local copies.

```yaml
status: accepted
minimum_tinygo: "0.42"
stdlib_http:
  router: github.com/shibukawa/tinygodriver/httpmux
  required_semantics: "Go 1.22+ method and path patterns"
network_driver: github.com/shibukawa/tinygodriver/netdev
verification:
  - TinyGo build and package target tests reject unsupported versions
  - examples/httpserver builds with TinyGo 0.42
  - ServeMux method, wildcard, PathValue, 405, and Allow fixtures pass
source: https://github.com/shibukawa/tinygodriver
```
