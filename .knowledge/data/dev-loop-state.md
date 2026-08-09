---
id: data:dev-loop-state
type: data
title: Developer Loop State
---
The record api:cli-dev publishes on every phase transition, read by requirement:dev-error-overlay and shown on the requirement:dev-console index.

```yaml
fields:
  build: an identity that changes whenever the served application changes, so a page can tell whether it is stale
  phase: the api:cli-dev phase in progress or last completed, using the same names policy:cli-progress-reporting prints
  status: starting, healthy, or failed
  diagnostic:
    present: only when status is failed
    text: the diagnostic unchanged, as the terminal received it
    location: file, line, and column when the diagnostic carried them
  since: the transition time, so a stale page can say how long ago
rules:
  - one current record, not a history; the loop's past is the terminal scrollback and requirement:dev-telemetry-viewer
  - the record is the same one whether the failure came from generation, migration, a build, or a process exit, so a reader handles one shape
  - status healthy means the application process is running, not that a request would succeed
  - the record holds no configuration value, no environment variable, and no path outside the project root
  - it lives in memory in the pw process and is never written into the project
producer: api:cli-dev
consumers:
  - requirement:dev-error-overlay through flow:dev-overlay-delivery
  - requirement:dev-console-launcher, which reads status off the same stream and shows nothing else from the record
  - requirement:dev-console index
```
