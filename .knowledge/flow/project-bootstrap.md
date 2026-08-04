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
    - id: choose
      actor: actor:application-developer
      action: answer the decision:interactive-project-bootstrap wizard, which a project name argument seeds rather than skips; --yes or a session with no terminal takes the shortcut flags instead
    - id: validate
      actor: system:pw-cli
      action: validate project name and destination
    - id: scaffold
      action: write concept:project-layout handwritten files, including requirement:nested-html-templates document shell, ui:starter-landing-page, and data:migration-source version 1
    - id: toolchain
      action: record project.toolchain and emit the flow:handler-registration mux for it, adding TinyGo to Devbox when selected
    - id: generate
      action: invoke api:cli-generate
    - id: optional-css
      action: pin decision:tailwind-host-toolchain in Devbox and scaffold the CSS entry when requested
    - id: summarize
      action: report the created sources under policy:cli-progress-reporting, counting rather than listing the generated artifacts
      output: cd, devbox shell, and api:cli-dev commands
  failure:
    default: report failing phase and leave no partial replacement of existing files
    canceled_wizard: write nothing and exit successfully
```
