---
id: requirement:tinygodriver-adoption
type: requirement
title: tinygodriver Adoption
---
Petitweb consumes reusable TinyGo networking compatibility from system:tinygodriver and must not retain duplicate implementations.

```yaml
ownership:
  authoritative_source: system:tinygodriver
  petitweb_role: consumer
imports:
  router: github.com/shibukawa/tinygodriver/httpmux
  reverse_proxy: github.com/shibukawa/tinygodriver/httprevproxy
  host_network_driver: github.com/shibukawa/tinygodriver/netdev
repository_boundary:
  - no local httpmux, httprevproxy, or netdev implementation
  - application examples import system:tinygodriver directly
dependency_rule:
  - pin a tested tinygodriver version in go.mod
  - do not vendor or fork these packages inside this repository
backward_compatibility:
  legacy_import_paths: unsupported
  forwarding_packages: forbidden
acceptance:
  - no tracked local implementation remains for httpmux, httprevproxy, or netdev
  - no source or generated project imports the removed petitweb-go package paths
  - host Go tests pass with httpmux resolving to net/http.ServeMux
  - TinyGo HTTP build and route fixtures pass with httpmux and netdev
  - reverse proxy fixtures pass against the external httprevproxy package
```
