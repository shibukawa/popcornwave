---
id: flow:project-bootstrap
type: flow
title: Project Bootstrap Flow
---
Project bootstrap turns an empty directory into a tested Petitweb service.

```yaml
flow:
  trigger: actor:application-developer invokes api:cli-init
  steps:
    - id: validate
      actor: system:petitweb-cli
      action: validate destination and module input
    - id: scaffold
      action: write concept:project-layout handwritten files
    - id: generate
      action: invoke api:cli-generate
    - id: verify
      action: run host Go tests
    - id: summarize
      output: commands for api:cli-dev and api:cli-build
  failure:
    default: report failing phase and leave no partial replacement of existing files
```
