---
id: flow:project-bootstrap
type: flow
title: Project Bootstrap Flow
---
Project bootstrap turns a project name into a runnable Popcorn Wave application and a reproducible Devbox development environment.

```yaml
flow:
  trigger: actor:application-developer invokes api:cli-init
  steps:
    - id: validate
      actor: system:pw-cli
      action: validate project name and destination
    - id: scaffold
      action: write concept:project-layout handwritten files, including requirement:nested-html-templates document shell and data:migration-source version 1
    - id: generate
      action: invoke api:cli-generate
    - id: optional-css
      action: pin decision:tailwind-host-toolchain in Devbox and scaffold the CSS entry when requested
    - id: summarize
      output: cd, devbox shell, and api:cli-dev commands
  failure:
    default: report failing phase and leave no partial replacement of existing files
```
