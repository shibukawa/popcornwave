---
id: flow:handler-scaffolding
type: flow
title: Handler Scaffolding Flow
---
Handler scaffolding turns a method and path into a registered route, its handler source, and its generated mapping code, without an operator editing a mux.

```yaml
flow:
  trigger: actor:application-developer invokes api:cli-new handler
  steps:
    - id: locate
      actor: system:pw-cli
      action: resolve the data:project-config project root by walking up from the working directory, and read project.toolchain
    - id: place
      action: preselect the working directory as the destination package when it lies under a decision:explicit-generation-sources handler entry, and otherwise leave the destination unanswered
    - id: ask
      actor: actor:application-developer
      action: answer the decision:post-init-scaffold-wizard steps for package, method, path, name, response kind, and request input
    - id: validate
      action: check the pattern against rule:static-route-discovery and against the routes the target package already registers
    - id: plan
      action: compute the handler source, the optional .pw.html template, and the flow:handler-registration mux when the package is new
    - id: review
      actor: actor:application-developer
      action: accept or cancel the planned files
    - id: write
      action: create the planned files atomically, refusing any existing destination
    - id: generate
      action: invoke api:cli-generate so the {source-base}_pw_gen.go artifacts exist before the next build
    - id: summarize
      output: the created paths, plus the concept:application-entry-point import to add when the handler package is new
  failure:
    duplicate_route: name the file that already registers the pattern and write nothing
    existing_file: name the path and write nothing
    canceled_wizard: write nothing and exit successfully
    generation_error: report the failing source; the written handler stays, because it is the source the operator has to fix
```
