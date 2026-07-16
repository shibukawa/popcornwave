---
id: requirement:contrib-html-template
type: requirement
title: Generated HTML Templates
---
contrib/htmltemplate compiles a Petitweb-specific typed template language and Go data models into reflection-free HTML and JSON writer functions.

```yaml
runtime_package: contrib/htmltemplate
generator: system:petitweb-cli
flow: flow:template-generation
generation_entrypoints:
  - api:cli-generate
  - package-local go:generate directive invoking Petitweb CLI
input:
  templates: configured HTML template source files
  binding: template name plus same-package Go data type in data:project-config
generated_api:
  - "Render<Name>(io.Writer, DataType) error"
  - "Write<Name>(http.ResponseWriter, status, DataType) error"
  - "Write<Name>JSON(io.Writer, DataType) error"
syntax_required:
  - statically typed nested struct field interpolation
  - if and else
  - optional pointer and value scopes
  - loops over arrays and slices with typed index and element variables
  - loops over maps with typed key and value variables and deterministic key ordering
  - named template inclusion with statically known names
  - typed comparisons and boolean operators
  - generator-registered typed helper functions
  - JSON expression for frontend state embedded in a safe application/json script context
type_analysis:
  - use go/types during generation
  - reject unknown or unexported field access with template source position
  - infer loop element, map key, map value, pointer, and nested field types
  - reject unsupported operations before Go compilation
json_codegen:
  - emit direct writers for reachable structs, pointers, arrays, slices, and string-keyed maps
  - honor json field names, omission, ignored fields, and supported scalar types
  - encode map keys deterministically
  - reject any, interfaces, functions, channels, complex values, and recursive object graphs in the first release
  - share generated primitive escaping helpers with system:httpbinder when a stable API exists
security: policy:template-escaping
priority: deferred until generated JSON encoding scope is implemented and verified
non_goals:
  - runtime parsing
  - runtime reflection
  - dynamic template names
  - arbitrary method invocation
  - syntax or output compatibility with standard html/template
  - arbitrary json.Marshaler execution in the first release
```
