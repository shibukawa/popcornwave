---
id: flow:template-generation
type: flow
title: HTML Template Generation Flow
---
Template generation validates .pw.html syntax, typed parameters, Go model access, loops, JSON type graphs, and output context before emitting TinyGo-safe code beside the template.

```yaml
flow:
  trigger: api:cli-generate finds a .pw.html source
  steps:
    - id: parse
      action: parse Popcorn Wave static template syntax without requiring standard html/template compatibility
    - id: resolve-model
      action: load the declared Go package and resolve exported fields, pointers, loop element types, map key and value types, and permitted helpers with go/types
    - id: resolve-json
      action: build the reachable JSON encoding graph for the page model and JSON expressions
    - id: escape-analysis
      action: assign each interpolation an HTML, attribute, URL, JavaScript, CSS, JSON script data, or text context
    - id: validate
      action: reject missing fields, invalid loops, unsupported JSON types, recursive inclusion or data graphs, and ambiguous contexts
    - id: emit
      action: generate direct typed field access, native Go loops, context escaping, and typed JSON writer calls
    - id: format
      action: format and atomically update policy:generated-artifacts
  failure:
    default: report source file, line, column, and template expansion stack
specialization: flow:error-template-generation uses the same parser, escaping, and atomic output rules
```
