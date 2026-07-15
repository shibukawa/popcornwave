---
id: policy:generated-artifacts
type: policy
title: Generated Artifact Ownership
---
Generated binder and OpenAPI files are reproducible build inputs owned exclusively by api:cli-generate.

```yaml
files:
  - httpbinder_gen.go
  - httpbinder_openapi_gen.go
  - petitweb_template_gen.go
rules:
  - include a generated-code header
  - never edit manually
  - commit to version control for reviewable API drift and reproducible TinyGo builds
  - api:cli-generate --check must pass in CI
  - replace atomically
authority:
  source: handwritten Go types, handlers, literal routes, and HTML templates
  generator: pinned system:httpbinder module version
```
