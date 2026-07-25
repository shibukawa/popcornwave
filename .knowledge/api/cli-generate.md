---
id: api:cli-generate
type: api
title: pw generate
---
pw generate scans Go, .pw.html, and .pw.sql sources and emits all required application mapping and codec code beside its source.

```yaml
usage: pw generate [--check]
inputs:
  - pw.Parse[T] call sites
  - route registrations
  - .pw.html files
  - .pw.sql files
  - reachable JSON types
flow: flow:generation-pipeline
artifacts:
  - request binding
  - OpenAPI fragments
  - typed HTML renderers
  - context-based SQL functions
  - optimized JSON codecs
  - optional generated tests
check_mode:
  writes: none
  failure: generated content differs or is missing
behavior:
  - discover all eligible sources without project include lists
  - use system:tinybind route and call analysis behind the pw API
  - process sources and packages in stable lexical order
  - stop on parse or generation error
  - format generated Go source
  - replace destination files atomically after all generation succeeds
  - emit {source-base}_pw_gen.go beside each source
```
