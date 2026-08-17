---
id: policy:generated-artifacts
type: policy
title: Generated Artifact Ownership
---
Generated Go files are reproducible application build inputs owned by api:cli-generate.

```yaml
applies_to: an application project; a concept:component-package inverts the version-control rule below and keeps every other one, per decision:committed-package-artifacts
scope_of_this_file: >
  generated Go, which is one artifact class; the ownership rules here are general
  and the **/*_pw_gen.go ignore line is this class's instance of them
other_artifact_classes:
  generated_public_assets: decision:generated-public-asset-version-control, for the Tailwind stylesheet and the extracted component assets under public/generated
  why_stated_elsewhere: >
    the ignore line here names a Go file pattern no .css or .js can match, so a
    second class needs its own line rather than a reading of this one
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
  - api:cli-check must pass in CI
  - replace atomically
  - delete stale package-local SQL runtime files removed by decision:tinybind-sql-runtime
authority:
  source: handwritten Go types, handlers, literal routes, .pw.html, and .pw.sql
  generator: pinned system:tinybind version
```
