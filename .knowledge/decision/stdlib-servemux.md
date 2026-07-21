---
id: decision:stdlib-servemux
type: decision
title: Standard ServeMux Routing
---
Petitweb applications register Go 1.22+ ServeMux method-and-path patterns through system:tinygodriver so one import works on host Go and the decision:tinygo-042-baseline target.

```yaml
status: accepted
router:
  package: github.com/shibukawa/tinygodriver/httpmux
  host: alias of net/http.ServeMux
  tinygo: TinyGo-compatible ServeMux implementation
minimum_go_syntax: "Go 1.22 route patterns"
example: 'mux.HandleFunc("POST /users/{id}", updateUser)'
discovery: rule:static-route-discovery
non_goals:
  - local ServeMux compatibility package
  - custom router DSL
rationale:
  - compatible with system:httpbinder route analysis
  - requirement:tinygodriver-adoption centralizes reusable TinyGo compatibility outside Petitweb
  - host builds retain standard net/http.ServeMux behavior
  - familiar Go handler model
```
