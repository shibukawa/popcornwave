---
id: flow:project-diagnosis
type: flow
title: Project Diagnosis Flow
---
Configuration diagnosis turns a project and one or more environment tokens into data:diagnostic-report, reading only files.

```yaml
flow:
  trigger: actor:application-developer invokes api:cli-doctor
  steps:
    - id: locate
      actor: system:pw-cli
      action: resolve the data:project-config project root and fail outside one
    - id: resolve-tokens
      action: take the --env tokens, defaulting to the APP_ENV of the pw process and then dev, and expand all from the project-local config file names
    - id: check-generated
      action: run the api:cli-generate drift check, because the configuration view is read from generated metadata
      failure: record the drift finding and mark the configuration and feature sections approximate
    - id: read-metadata
      action: collect binding prefixes, keys, and typed defaults from the generated configbind sources and the popcornweb module in the build list
    - id: resolve-graph
      action: resolve the import graph of data:project-config project.main and analyze the registration call sites it reaches
      failure: analyze the packages that parse and record the rest as one error finding
    - id: merge-per-token
      action: for each token, select the TOML candidate through policy:config-file-resolution and merge it over the typed defaults
    - id: mark-limits
      action: mark every field whose deployed value would come from an environment variable or CLI, and every registration argument that is not statically resolvable
    - id: resolve-features
      action: derive the feature, middleware, database, and registration views from the merged configuration and the graph
    - id: read-routes
      action: load data:route-table and the template sources
      failure: skip the route and template checks as limits rather than reporting collisions the table cannot back up
    - id: connect
      action: run the rule:storage-checks online set only when the online option was given, bounded by the configured timeouts
      failure: record the connection failure as a finding and suppress the checks behind it
    - id: evaluate
      action: run every decision:shared-check-catalog check whose declared inputs this run built, per token, skipping any check whose input is marked as a limit
    - id: render
      output: data:diagnostic-report in the selected format
    - id: exit
      action: exit nonzero on any error finding, or on any warning under strict
  failure:
    default: report and continue; only a missing project root and an unparsable option stop before the report
    partial: an incomplete section is named with its reason, and every check it suppressed is listed
    never: no step builds the application, starts a process, or writes a file, and none opens a connection without the online option
dfd:
  boundary: system:pw-cli
  actors:
    - actor:application-developer
  stores:
    - data:project-config
    - data:migration-source
  flows:
    - from: data:project-config
      to: api:cli-doctor
      purpose: project root, main package, tooling state, and generate purposes
    - from: api:cli-generate
      to: api:cli-doctor
      data: generated configbind metadata and drift state
    - from: api:cli-doctor
      to: rule:configuration-advisories
      data: merged configuration per token, plus the registration inventory
    - from: api:cli-doctor
      to: actor:application-developer
      data: data:diagnostic-report
```
