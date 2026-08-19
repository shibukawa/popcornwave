---
id: requirement:editor-code-actions
type: requirement
title: Template Code Actions
---
A diagnostic whose repair is mechanical offers that repair, and the offer exists only where requirement:editor-diagnostics already reported the problem.

```yaml
status: proposed
stage: 4 of vision:editor-support
server: requirement:pw-language-server
gate: every action repairs a decision:shared-check-catalog finding, so the editor offers nothing api:cli-generate would not have complained about
actions:
  missing_import: add the import a referenced component or type needs, chosen from the module's own package list
  undeclared_component: create the referenced component declaration, with the parameters the call site passes
  missing_dynamo_tag: add the struct tag a key clause names but the bound type does not declare
  unsatisfied_sql_contract: adjust the result type to what the statement returns, or name why it cannot be adjusted
  out_of_purpose_source: move the file into a directory data:project-config declares, per decision:explicit-generation-sources
  wrong_output_type: replace an output type the root keyword does not allow with one it does
  add_slot: declare a slot a caller fills
excluded:
  layout: requirement:template-formatting owns layout; an action never reflows
  rename: requirement:declaration-rename, because a name crosses files an action should not silently touch
  generation: an action never runs api:cli-generate, per policy:editor-tool-execution
  invented_code: an action fills a shape the checks already describe and never guesses a body
scope:
  writes: the open document, except for the file-creating and file-moving actions, which are previewed as a workspace edit
  project_required: an action needing resolution is not offered without a loaded project, rather than offered and failing
acceptance:
  - every offered action names the diagnostic it repairs
  - applying an action clears the diagnostic it named and introduces no new one
  - no action is offered on a document with no diagnostic
  - a file-creating action shows the file it would create before creating it
  - an action is never offered in a *_pw_gen.go
```
