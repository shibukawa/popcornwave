---
id: flow:template-generation
type: flow
title: HTML Template Generation Flow
---
Template generation validates syntax, model access, and output context on the host before emitting TinyGo-safe code.

```yaml
flow:
  trigger: api:cli-generate finds requirement:contrib-html-template configuration
  steps:
    - id: parse
      action: parse template syntax with host Go tooling
    - id: resolve-model
      action: resolve exported fields and permitted helpers from the declared data type
    - id: escape-analysis
      action: assign each interpolation an HTML, attribute, URL, JavaScript, CSS, or text context
    - id: validate
      action: reject missing fields, invalid helpers, recursive inclusion, and ambiguous contexts
    - id: emit
      action: generate direct typed field access and escaping calls
    - id: format
      action: format and atomically update policy:generated-artifacts
  failure:
    default: report source file, line, column, and template expansion stack
```
