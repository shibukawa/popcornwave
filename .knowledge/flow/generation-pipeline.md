---
id: flow:generation-pipeline
type: flow
title: Binding Generation Pipeline
---
The generation pipeline converts application Go sources into TinyGo-safe binding and OpenAPI source artifacts.

```yaml
flow:
  trigger: api:cli-generate reads data:project-config
  steps:
    - id: resolve
      action: resolve and sort configured package directories
    - id: analyze
      actor: system:httpbinder
      action: load requirement:httpbinder-extensible-route-analysis adapters, then parse structs, explicit Bind and Write calls, and rule:static-route-discovery
    - id: emit-binding
      output: httpbinder_gen.go
    - id: emit-openapi
      condition: OpenAPI enabled
      output: httpbinder_openapi_gen.go
    - id: emit-templates
      condition: templates configured
      action: run flow:template-generation
      output: petitweb_template_gen.go
    - id: compare-or-commit
      action: compare in check mode or atomically replace policy:generated-artifacts
  failure:
    default: no destination artifact is partially written
```
