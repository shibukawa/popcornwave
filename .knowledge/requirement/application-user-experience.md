---
id: requirement:application-user-experience
type: requirement
title: Popcorn Wave Application User Experience
---
This is the authoritative application-facing specification; when an older catalog decision conflicts, this requirement and its referenced concepts take precedence.

```yaml
product:
  vision: vision:popcorn-wave
  package_boundary: concept:public-package-boundaries
  entry_point: concept:application-entry-point
cli:
  system: system:pw-cli
  commands:
    - api:cli-init
    - api:cli-generate
    - api:cli-build
    - api:cli-dev
  test_command: use go test ./...; no initial pw test
project:
  layout: concept:project-layout
  tooling_config: data:project-config
http:
  mux: api:serve-mux
  registration: flow:handler-registration
  lifecycle: api:application-lifecycle
  middleware: policy:web-middleware
configuration:
  API: api:runtime-configuration
  model: decision:independent-runtime-config-bindings
  subcommands: api:subcommands
generation:
  pipeline: flow:generation-pipeline
  artifacts: policy:generated-artifacts
  html: flow:template-generation
  sql: flow:sql-generation
handler_api:
  request: api:request-binding
  html: api:html-response
  api: api:api-response
  stream: api:typed-stream
  problem: api:problem-response
  transaction: api:transaction-runner
runtime:
  context: api:request-context-accessors
  engine: system:tinybind
  tinygo_compatibility: system:tinygodriver
```
