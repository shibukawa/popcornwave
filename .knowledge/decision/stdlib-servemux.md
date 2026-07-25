---
id: decision:stdlib-servemux
type: decision
title: Standard ServeMux Routing
---
Popcorn Wave applications register Go 1.22+ ServeMux method-and-path patterns through api:serve-mux so one pw import works on host Go and the decision:tinygo-042-baseline target.

```yaml
status: accepted
router:
  public: api:serve-mux
  implementation: github.com/shibukawa/tinygodriver/httpmux
  host: net/http ServeMux-compatible fallback
  tinygo: tinygodriver ServeMux-compatible implementation
minimum_go_syntax: "Go 1.22 route patterns"
example: 'mux.HandleFunc("POST /users/{id}", updateUser)'
discovery: rule:static-route-discovery
non_goals:
  - local ServeMux compatibility package
  - custom router DSL
rationale:
  - compatible with system:tinybind route analysis
  - requirement:tinygodriver-adoption centralizes reusable TinyGo compatibility outside Popcorn Wave
  - host builds retain standard net/http.ServeMux behavior
  - familiar Go handler model
```
