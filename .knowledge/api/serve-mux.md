---
id: api:serve-mux
type: api
title: ServeMux API
---
pw exposes Go 1.22 ServeMux behavior through one host and TinyGo-compatible routing surface.

```yaml
surface:
  - NewServeMux() *ServeMux
  - ServeMux.Handle(pattern string, handler http.Handler)
  - ServeMux.HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
compatibility:
  patterns: Go 1.22 method and path patterns
  path_parameters: Request.PathValue behavior
  host_go: delegates or aliases to net/http ServeMux behavior
  tinygo: uses system:tinygodriver compatible implementation
example: 'mux.HandleFunc("GET /users/{id}", showUser)'
scope:
  owns:
    - route matching
    - method matching
    - path parameter extraction
  excludes:
    - middleware
    - route metadata
    - authorization semantics
rule: ordinary net/http handlers remain valid
```
