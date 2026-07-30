---
id: flow:generation-pipeline
type: flow
title: Application Generation Pipeline
---
The generation pipeline converts application Go, HTML, and SQL sources into deterministic Go artifacts that run without target-side reflection.

```yaml
flow:
  trigger: api:cli-generate reads data:project-config
  steps:
    - id: resolve
      action: walk each decision:explicit-generation-sources purpose, sort matching source paths, and warn about sources found outside the purpose that owns their kind
    - id: analyze
      actor: system:tinybind
      action: parse route registrations, pw.Parse calls, response calls, reachable types, .pw.html, and .pw.sql sources
    - id: select
      action: drop artifacts whose purpose does not list the directory that produced them
    - id: emit-go
      outputs:
        - request binders and OpenAPI fragments
        - flow:template-generation renderers
        - flow:sql-generation query functions
        - optimized JSON codecs
        - optional generated tests
      naming: "{source-base}_pw_gen.go beside the source"
    - id: compare-or-commit
      action: compare in check mode or atomically replace policy:generated-artifacts
  failure:
    default: no destination artifact is partially written
```
