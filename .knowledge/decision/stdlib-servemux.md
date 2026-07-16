---
id: decision:stdlib-servemux
type: decision
title: Standard ServeMux Routing
---
Petitweb applications register routes directly with Go 1.22+ net/http ServeMux method-and-path patterns on host Go and the decision:tinygo-042-baseline target.

```yaml
status: accepted
router:
  host: net/http.ServeMux
  tinygo: net/http.ServeMux
minimum_go_syntax: "Go 1.22 route patterns"
example: 'mux.HandleFunc("POST /users/{id}", updateUser)'
discovery: rule:static-route-discovery
non_goals:
  - contrib/httpmux compatibility package
  - custom router DSL
rationale:
  - compatible with system:httpbinder route analysis
  - TinyGo 0.42 updates bundled HTTP routing to the required semantics
  - no runtime router dependency
  - familiar Go handler model
```
