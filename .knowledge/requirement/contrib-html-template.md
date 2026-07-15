---
id: requirement:contrib-html-template
type: requirement
title: Generated HTML Templates
---
contrib/htmltemplate compiles a safe Go-template subset and typed data models into reflection-free render functions.

```yaml
runtime_package: contrib/htmltemplate
generator: system:petitweb-cli
flow: flow:template-generation
input:
  templates: "*.html.tmpl"
  binding: template name plus same-package Go data type in data:project-config
generated_api:
  - "Render<Name>(io.Writer, DataType) error"
  - "Write<Name>(http.ResponseWriter, status, DataType) error"
syntax_required:
  - escaped field interpolation
  - if and else
  - with
  - range over arrays, slices, and maps with deterministic map-key ordering
  - named template inclusion with statically known names
  - built-in comparisons and boolean operators
  - generator-registered typed helper functions
security: policy:template-escaping
non_goals:
  - runtime parsing
  - runtime reflection
  - dynamic template names
  - arbitrary method invocation
  - full html/template compatibility in first release
evidence: https://tinygo.org/docs/reference/lang-support/stdlib/#htmltemplate
```
