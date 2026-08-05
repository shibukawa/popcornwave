---
id: policy:generated-artifacts
type: policy
title: Generated Artifact Ownership
---
Generated Go files are reproducible application build inputs owned by api:cli-generate.

```yaml
applies_to: an application project; a concept:component-package inverts the version-control rule below and keeps every other one, per decision:committed-package-artifacts
pattern:
  ordinary: "{source-base}_pw_gen.go"
location: beside the owning Go, .pw.html, or .pw.sql source
contents:
  - request binders and OpenAPI fragments
  - typed HTML renderers
  - typed SQL functions
  - optimized serializers
  - optional generated tests
  - main bootstrap blank imports for document and public registration
rules:
  - include a generated-code header
  - never edit manually
  - never begin generated filenames with an underscore
  - exclude from version control with the init-scaffolded **/*_pw_gen.go ignore rule, in an application project only
  - recreate during application builds before Go compilation
  - api:cli-generate --check must pass in CI
  - replace atomically
  - delete stale package-local SQL runtime files removed by decision:tinybind-sql-runtime
authority:
  source: handwritten Go types, handlers, literal routes, .pw.html, and .pw.sql
  generator: pinned system:tinybind version
```
