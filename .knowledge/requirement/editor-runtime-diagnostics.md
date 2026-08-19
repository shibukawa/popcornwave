---
id: requirement:editor-runtime-diagnostics
type: requirement
title: Runtime Findings in the Editor
---
A failure that only happens when the application runs is reported in the editor at the template position that produced it, so the class of problem static analysis cannot reach is not the one class the developer debugs by reading logs.

```yaml
status: the dev-loop source is implemented, opt-in, over the requirement:dev-console loop state; the requirement:browser-report-ingest source is unimplemented
stage: 3 of vision:editor-support
sources:
  dev_loop: a template render failure, a query failure, and a panic observed by api:cli-dev
  browser: the requirement:browser-report-ingest records of data:browser-report-record, which carry client-side failures a server never sees
  console: the requirement:dev-console already collects both, so the editor subscribes to it rather than to the application
transport:
  from: the requirement:dev-console listener of decision:dev-console-consolidation, over its own endpoint
  when: only while api:cli-dev is running and the developer opted in, because policy:editor-tool-execution never starts the loop implicitly
  never: no report leaves the machine, per the data rules of policy:editor-tool-execution
positioning:
  server_side: requirement:template-source-positions, which is what turns a generated stack frame into a template line
  client_side: the requirement:derived-asset-pipeline source map, which is what turns a bundled frame into a TypeScript line
  unmappable: reported against the project rather than dropped, because a report with no position is still a report
presentation:
  separate_from_static: a distinct diagnostic source, so a runtime finding is never confused with one api:cli-generate would produce
  lifetime: cleared when the dev loop restarts, because a finding from a previous process describes code that may no longer exist
  volume: bounded per position, so a failing loop does not fill the Problems view with one repeated error
non_goals:
  - a telemetry surface in the editor; requirement:dev-telemetry-viewer stays a browser tool, per vision:editor-support
  - reports from a deployed environment, which requirement:browser-report-ingest handles on the server
  - starting api:cli-dev to obtain a report
acceptance:
  - a panic in a generated renderer appears against the .pw.html line
  - a browser report appears against the TypeScript line rather than the bundle
  - the findings clear when the dev loop restarts
  - no finding appears when the dev loop is not running
  - a runtime finding and a static one on the same line are distinguishable
```
