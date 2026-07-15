---
id: api:cli-build
type: api
title: petitweb build
---
petitweb build produces a TinyGo application executable after deterministic generation and compatibility checks.

```yaml
usage: "petitweb build [--target <tinygo-target>] [--output <path>]"
steps:
  - run api:cli-generate in check-equivalent mode after updating artifacts
  - run rule:tinygo-runtime-compatibility preflight
  - create output parent directory
  - execute "tinygo build -o <output> [-target <target>] <package>"
defaults:
  package: data:project-config dev package
  target: data:project-config build target; empty means TinyGo host default
  output: data:project-config build output
failure:
  - preserve previous successful output
  - return compiler diagnostics and nonzero status
```
