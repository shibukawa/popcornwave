---
id: system:tinybind
type: system
title: TinyBind
---
TinyBind is the generated binding, configuration, response, validation, streaming, and OpenAPI engine wrapped by the Popcorn Wave public APIs.

```yaml
module: github.com/shibukawa/tinybind-go
html_template_baseline: v0.1.15
public_wrappers:
  - api:request-binding
  - api:html-response
  - api:api-response
  - api:typed-stream
  - api:problem-response
  - api:runtime-configuration
generator:
  extensible_analysis: requirement:httpbinder-extensible-route-analysis
  openapi:
    - package fragments register during import
    - AssembleOpenAPI returns deterministic merged JSON and YAML
  configuration:
    - generated definitions register during import
    - ScaffoldTOML and ScaffoldEnv merge every package definition
  html:
    - .pw.html components generate immutable htmlbind.Fragment values
    - components with an unnamed slot also generate htmlbind.Wrapper binders
    - api:render-html-chain composes wrappers around a leaf
constraints:
  - generator executes with host Go
  - generated mapping path avoids runtime field reflection
  - route discovery analyzes same-package registrations recognized by versioned adapters
  - normal handwritten application code does not import TinyBind
compatibility:
  html_v0_1_15: generated HTML APIs are not source-compatible with earlier direct-writer output
```
