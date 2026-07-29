---
id: flow:dev-telemetry-capture
type: flow
title: Development Telemetry Capture Flow
---
requirement:dev-telemetry-viewer captures one developer-loop run from viewer startup to browser inspection.

```yaml
flow:
  trigger: api:cli-dev starts
  steps:
    - id: listen
      action: bind the loopback viewer listener before the application process starts
      failure: report the address and continue the developer loop without a viewer
    - id: inject
      action: set the OTLP endpoint, protocol, and service name on the application process
      skip: an exported endpoint suppresses injection and viewer startup
    - id: report
      action: print the viewer URL through policy:startup-summary
    - id: sample
      action: bind process health sampling to the started application pid
    - id: emit
      action: requirement:contrib-otel opens spans and api:logger produces records under policy:log-emission
    - id: export
      action: flow:telemetry-export batches and posts traces and logs to the viewer receiver
    - id: tee
      action: write the same records to stdout so the developer loop stream stays populated
    - id: store
      action: the viewer holds records in bounded memory keyed by trace and span
    - id: inspect
      action: the developer reads spans and correlated records in the browser
  restart:
    - the application process stops and restarts on change
    - the viewer keeps its listener, its port, and everything already captured
    - process health sampling rebinds to the new pid
  shutdown:
    - stop the viewer with the developer loop
    - discard stored telemetry
```
