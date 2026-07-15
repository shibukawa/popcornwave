---
id: rule:static-route-discovery
type: rule
title: Static Route Discovery
---
Routes used for generated OpenAPI must be statically discoverable in the same Go package as their handlers.

```yaml
required:
  - Handle or HandleFunc registration recognized by requirement:httpbinder-extensible-route-analysis
  - literal route pattern
  - Go 1.22 method-and-path form preferred
  - named, inline, or ServeHTTP handler body visible to system:httpbinder
  - explicit Bind, Write, NewStream, and error constructor calls in the handler body
unsupported:
  - dynamically concatenated patterns
  - registration APIs without a built-in or configured analysis adapter
  - registrations hidden behind a Petitweb route DSL
  - cross-package handler bodies whose types cannot be inspected
consequence: unsupported routes may run but are absent or incomplete in generated OpenAPI and must fail api:cli-check
```
