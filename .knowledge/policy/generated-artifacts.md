---
id: policy:generated-artifacts
type: policy
title: Generated Artifact Ownership
---
Generated Go files are reproducible build inputs owned by api:cli-generate.

```yaml
pattern:
  ordinary: "{source-base}_pw_gen.go"
location: beside the owning Go, .pw.html, or .pw.sql source
contents:
  - request binders and OpenAPI fragments
  - typed HTML renderers
  - typed SQL functions
  - optimized serializers
  - optional generated tests
rules:
  - include a generated-code header
  - never edit manually
  - never begin generated filenames with an underscore
  - commit to version control for reviewable API drift and reproducible TinyGo builds
  - api:cli-generate --check must pass in CI
  - replace atomically
authority:
  source: handwritten Go types, handlers, literal routes, .pw.html, and .pw.sql
  generator: pinned system:tinybind version
```
