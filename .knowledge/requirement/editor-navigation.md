---
id: requirement:editor-navigation
type: requirement
title: Cross-Language Navigation
---
A name that crosses the boundary between a template source, its generated Go, and the handwritten Go that calls it is navigable in one jump, because that boundary is where the framework's indirection lives.

```yaml
status: proposed
stage: 3 of vision:editor-support
server: requirement:pw-language-server
jumps:
  component_to_component: a component reference in a .pw.html body to its declaration, in the same file or another one
  declaration_to_call: a component, statement, or dynamo declaration to the Go call sites of its generated function
  call_to_declaration: a generated function call in handwritten Go to the .pw.* declaration that produced it, skipping the *_pw_gen.go in between
  external_binding: an external, external async, or external live declaration to the Go function it binds
  page_tree: a concept:page-tree page or layout to its route, and a route to its page, per flow:page-route-generation
  action: a server action reference in a template to its Go handler, per api:server-action
  sql_table: a table name in a .pw.sql or a .pw.dynamo table clause to the data:migration-source statement that creates it
  dynamo_attribute: an attribute in a key clause to the dynamo-tagged struct field that declares it
hover:
  declaration: the resolved signature, the generated Go function name, and the output type
  parameter: its declared type
  dynamo_attribute: the tag, its stored type, and its key role
generated_files:
  rule: a *_pw_gen.go is a waypoint, never a destination; navigation resolves through it to the source
  reason: policy:generated-artifacts makes it uncommitted output, so a bookmark into it is worthless
prerequisites:
  - a loaded project, because a call site is found by the same analysis api:cli-generate uses
  - generated output present for the Go direction, since the call site references a generated symbol
  - degraded answer when either is missing: report unavailable rather than a partial result
deferred:
  rename: a declaration name decides a Go symbol, a file name, and possibly a route, so renaming is a generator operation rather than an editor one
acceptance:
  - go-to-definition on a generated query call in a handler lands on the .pw.sql statement
  - go-to-definition on a component reference lands on its declaration and never on the generated fragment
  - find-references on a component declaration lists template references and Go call sites in one result
  - hover on a dynamo key attribute names the struct field and its stored type
  - a project with no generated output still navigates template to template
```
