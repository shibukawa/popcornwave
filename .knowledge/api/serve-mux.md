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
  path_parameters:
    behavior: Request.PathValue
    accessor: the module's own, httpbind.PathValue and fasthttpbind.PathValue, which carry the same name and take the transport first, so a rewrite moves the qualifier and nothing else
    not_ours: this framework declares no PathValue and should not; an earlier draft of this concept specified pw.PathValue(r, name) before the module had one, and a second spelling of the same read would be a name the transform has to know about for no gain
    still_open: routetree emits the decoder that performs the read, and it emits net/http only, so the second build has no decoder for a parameterised route until that emitter takes a transport
  host_go: delegates or aliases to net/http ServeMux behavior
  tinygo: uses system:tinygodriver compatible implementation
  other_backend:
    form: a third implementation behind this same name, assembling the router the generation target names and translating at registration
    default_target: the tinygodriver fasthttprouter fork rather than the upstream router, because the fork vendors its own request value and a handler built against it is not the type the upstream router accepts
    configurable: the target names the import, qualifier, type, registration function, and catch-all spelling, so an application on upstream fasthttp points it there instead
    absorbs:
      pattern_syntax: the catch-all spelling, rewritten from the Go 1.22 form to the router's
      method_split: one pattern string becomes the router's separate method and path arguments
      subtree: a pattern ending in a slash becomes the exact path plus a catch-all registration
      no_catch_all_spelling: a target declaring none rejects such a route by name rather than inventing one
      path_values: the router stores them where pw.PathValue reads them, so a handler using the accessor needs no rewrite
      behavior_flags: RedirectTrailingSlash, RedirectFixedPath, HandleMethodNotAllowed, and HandleOPTIONS are set to reproduce Go 1.22 behavior rather than left at the router's defaults
    cannot_absorb:
      precedence: Go 1.22 resolves overlapping patterns by specificity where a trie rejects them, and reproducing that would mean matching before dispatch, which gives up the reason to use the trie
      host_patterns: Go 1.22 matches on host, and the router does not
      exact_terminator: the Go 1.22 marker forcing an exact match has no counterpart
    consequence: a route table using only method, path, and named parameters translates; one relying on the three above needs a per-backend source, per decision:transport-source-transform
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
