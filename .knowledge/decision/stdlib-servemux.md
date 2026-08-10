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
scaffold_choice:
  owner: decision:interactive-project-bootstrap
  host_only: api:cli-init scaffolds net/http.ServeMux directly and records project.toolchain as go, which is the default
  tinygo: api:cli-init scaffolds api:serve-mux, taken by --tinygo or the wizard answer
  invariant: api:cli-generate discovers both mux types, so generated artifacts are identical
non_goals:
  - local ServeMux compatibility package
  - custom router DSL
rationale:
  - compatible with system:tinybind route analysis
  - requirement:tinygodriver-adoption centralizes reusable TinyGo compatibility outside Popcorn Wave
  - host builds retain standard net/http.ServeMux behavior
  - familiar Go handler model
```
