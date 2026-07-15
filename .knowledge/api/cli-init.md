---
id: api:cli-init
type: api
title: petitweb init
---
petitweb init creates a compilable starter project with typed health and validated echo endpoints plus generated-artifact conventions.

```yaml
usage: "petitweb init [directory] --module <module-path> [--force]"
inputs:
  directory: default current directory
  module: required unless derived from an existing go.mod
outputs:
  - data:project-config
  - concept:project-layout
  - starter GET /health endpoint
  - starter POST /echo endpoint demonstrating Bind, Validation, Field, Write, and WriteError
  - .gitignore entries for dist and temporary files
behavior:
  - validate module path and destination
  - refuse to overwrite nonempty destinations by default
  - create files atomically
  - run api:cli-generate
  - run host go test
force_rule: overwrite only files whose current content matches a known Petitweb template
exit:
  success: 0
  invalid_input_or_collision: nonzero with actionable path
```
