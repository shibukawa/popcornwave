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
      action: resolve configured globs and sort matching source paths
    - id: analyze
      actor: system:tinybind
      action: parse route registrations, pw.Parse calls, response calls, reachable types, .pw.html, and .pw.sql sources
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
