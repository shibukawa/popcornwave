---
id: concept:project-layout
type: concept
title: Generated Project Layout
---
The starter project separates the CLI configuration, server package, generated mapping, tests, and build output.

```yaml
layout:
  petitweb.yaml: data:project-config
  go.mod: Go module definition
  cmd/server/main.go: listener and route registration
  cmd/server/handlers.go: explicit system:httpbinder handler calls
  cmd/server/models.go: request and response structs
  cmd/server/handlers_test.go: net/http handler tests
  cmd/server/httpbinder_gen.go: generated binder and writer code
  cmd/server/httpbinder_openapi_gen.go: generated OpenAPI embed
  dist/: api:cli-build output ignored by Git
ownership:
  handwritten:
    - petitweb.yaml
    - go.mod
    - cmd/server/main.go
    - cmd/server/handlers.go
    - cmd/server/models.go
    - cmd/server/handlers_test.go
  generated: policy:generated-artifacts
```
