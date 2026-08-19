---
id: requirement:editor-inlay-hints
type: requirement
title: Template Inlay Hints
---
The types the generator resolved are shown in the template that produced them, because a .pw.* source names almost none of them and the developer otherwise reads the generated Go to learn what a binding holds.

```yaml
status: implemented at internal/pwlsp for the val, loop, and await families; a binding whose expression is not a call of a declaration has no resolved type and gets no hint
stage: 3 of vision:editor-support
server: requirement:pw-language-server
hints:
  binding: the resolved type of a val binding, which the source never writes
  loop_variable: the element type a loop binds, and the collection it came from
  sql_result: the column types a statement returns, shown against the result contract the header declares
  dynamo_attribute: the stored type and key role of an attribute a key clause names, from the dynamo-tagged struct field
  component_argument: the parameter name at a call site passing positionally
  async_state: the type a fallback renders with, per requirement:async-html-rendering, since it differs from the resolved state
  external_binding: the Go signature an external declaration binds to
rules:
  resolution_required: a hint appears only where the project model resolved the type; an unresolved position shows nothing rather than a guess
  no_project: no hints at all, matching the degraded answer requirement:pw-language-server gives elsewhere
  never_in_generated: a *_pw_gen.go gets none, because gopls already shows Go types there
  truncation: a long type is shortened with the full text on hover, so a hint never reflows the line
settings:
  granularity: each hint family above is separately switchable, because a developer who knows the SQL schema wants different noise from one learning it
  default: bindings, loop variables, and sql results on; argument names off, which is the VS Code convention for positional hints
acceptance:
  - a val binding of a statement call shows the statement's result type
  - a key clause attribute shows the struct field type that stores it
  - a document opened outside a project shows no hint
  - a hint family switched off produces no request work, not merely no display
```
