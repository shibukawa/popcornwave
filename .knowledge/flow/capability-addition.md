---
id: flow:capability-addition
type: flow
title: Capability Addition Flow
---
Capability addition turns a capability an existing project lacks into configuration, migrations, and development tooling, with the review screen as the only approval point.

```yaml
flow:
  trigger: actor:application-developer invokes api:cli-add
  steps:
    - id: locate
      actor: system:pw-cli
      action: resolve the data:project-config project root and fail outside one
    - id: probe
      action: detect installed capabilities from project files per requirement:incremental-project-capabilities
    - id: choose
      actor: actor:application-developer
      action: answer the decision:post-init-scaffold-wizard steps, offered only the capabilities the project lacks
    - id: resolve-dependency
      action: add a required missing capability to the plan, or stop when it is declined
    - id: plan
      action: compute every file to create, every configuration section to append, and the next free migration version
    - id: review
      actor: actor:application-developer
      action: accept or cancel the planned changes
    - id: write
      action: create files and append sections atomically, refusing any application-owned overwrite
    - id: generate
      action: invoke api:cli-generate when the capability added generated sources
    - id: summarize
      output: the follow-up commands, starting with api:migration-runner and any devbox shell the new packages need
  failure:
    conflict: name the file that proves the conflict and write nothing
    canceled_wizard: write nothing and exit successfully
    partial_write: forbidden; a failing step leaves the project as it was
```
