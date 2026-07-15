---
id: decision:stdlib-servemux
type: decision
title: Standard ServeMux Routing
---
Petitweb applications register routes with Go 1.22+ net/http ServeMux method-and-path patterns, using requirement:contrib-httpmux when the TinyGo implementation lacks those semantics.

```yaml
status: accepted
router:
  host: net/http.ServeMux or contrib/httpmux.ServeMux
  tinygo: contrib/httpmux.ServeMux when parity checks fail for bundled net/http
minimum_go_syntax: "Go 1.22 route patterns"
example: 'mux.HandleFunc("POST /users/{id}", updateUser)'
discovery: rule:static-route-discovery
rationale:
  - compatible with system:httpbinder route analysis
  - no runtime router dependency
  - familiar Go handler model
```
