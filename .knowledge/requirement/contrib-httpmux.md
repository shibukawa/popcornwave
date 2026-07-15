---
id: requirement:contrib-httpmux
type: requirement
title: TinyGo Standard ServeMux Compatibility
---
contrib/httpmux supplies Go 1.22+ net/http ServeMux routing semantics as an http.Handler when TinyGo's bundled ServeMux is incomplete.

```yaml
package: contrib/httpmux
public_api:
  - ServeMux implementing http.Handler
  - NewServeMux returns *ServeMux
  - ServeMux.Handle(pattern, http.Handler)
  - ServeMux.HandleFunc(pattern, func(http.ResponseWriter, *http.Request))
  - ServeMux.Handler(*http.Request) returns handler and matched pattern
pattern_compatibility:
  - '[METHOD ][HOST]/[PATH] syntax'
  - literal segments
  - single-segment '{name}' wildcards
  - terminal multi-segment '{name...}' wildcards
  - terminal '{$}' exact-end marker
  - GET also matches HEAD
  - segment-wise URL unescaping
required_behavior:
  - select the most specific matching pattern independent of registration order
  - panic at registration for invalid patterns, nil handlers, duplicates, and conflicting patterns
  - populate http.Request.PathValue for named wildcards before dispatch
  - return 405 and an Allow header when path and host match but method does not
  - preserve standard path cleaning, subtree slash redirects, host matching, and CONNECT handling
  - remain safe for concurrent dispatch after registration
integration:
  - usable directly as the handler passed to http.ListenAndServe
  - requirement:httpbinder-extensible-route-analysis provides its built-in route registration adapter
  - literal Handle and HandleFunc calls remain discoverable by rule:static-route-discovery
  - host Go and TinyGo use the same contrib implementation when deterministic parity matters
non_goals:
  - replacing net/http.DefaultServeMux or package-level http.Handle functions
  - GODEBUG=httpmuxgo121 legacy routing mode
  - route removal or mutation after serving begins
  - non-standard middleware or route grouping DSL
compatibility: behavioral subset of Go 1.22+ net/http.ServeMux; import path is intentionally different
reference: https://pkg.go.dev/net/http#ServeMux
```
