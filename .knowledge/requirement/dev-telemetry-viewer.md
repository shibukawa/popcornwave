---
id: requirement:dev-telemetry-viewer
type: requirement
title: Development Telemetry Viewer
---
api:cli-dev runs a loopback OpenTelemetry receiver and browser UI so requirement:contrib-otel spans and api:logger records are readable in the developer loop without an external collector.

```yaml
origin: system:localotelviewer
adoption: decision:dev-telemetry-viewer-adoption
scope: api:cli-dev only; never api:cli-build, api:test-run, or any deployed environment
default: enabled
configuration: data:project-config dev.otel
signals:
  traces: requirement:contrib-otel spans delivered by flow:telemetry-export
  logs: api:logger records routed by policy:log-emission
  metrics: the receiver accepts /v1/metrics, but requirement:contrib-otel emits none, so the view stays empty unless the application exports its own
  process: the viewer samples cpu, memory, threads, open files, and io of the process api:cli-dev started
listener:
  bind: loopback only
  port: dev.otel.port, defaulting to 0 so the operating system selects a free one
  reason: the endpoint is injected rather than written down, so no number has to be agreed in advance and concurrent projects never collide
  pin: a fixed port is set only when an external tool must find the receiver
  collision: a bound port fails the viewer, reports the address, and leaves the developer loop running
  url: printed once at startup beside the api:cli-dev identity provider report
  mount: receiver, snapshot API, and UI share one listener, so the printed URL is both the export target and the page to open
injection:
  mechanism: environment variables on the application process, which outrank TOML in data:loaded-configuration precedence
  variables:
    - OTEL_EXPORTER_OTLP_ENDPOINT set to the resolved viewer URL
    - OTEL_EXPORTER_OTLP_PROTOCOL set to http/protobuf
    - OTEL_SERVICE_NAME set to data:project-config project.name
  naming: OTLP conventions rather than pw-specific names, so any exporter finds them
  rule: injected only for the process api:cli-dev starts, never exported to the developer shell
  preserved: a variable the developer already exported is never overwritten
  developer_endpoint_wins:
    behavior: an exported OTEL_EXPORTER_OTLP_ENDPOINT skips both injection and viewer startup, with the reason printed
    reason: a viewer with no producer is a held port and an empty page
dual_log_emission:
  rule: while the viewer is enabled, a record reaches both the viewer and stdout
  reason: policy:log-emission routes exclusively, and moving every record out of the terminal would empty the developer loop stream api:cli-dev depends on
  scope: development only; exclusive routing is unchanged everywhere else
lifetime:
  start: before the application process, like requirement:contrib-devidp
  restart: the viewer keeps its listener, its port, and everything captured across regeneration, migration, rebuild, and restart
  process_sampling: rebound to the new process on every restart, because the previous pid is gone by then
  stop: with the developer loop
  retention: bounded in memory by dev.otel.max, discarded at shutdown, and never written into the project
packaging:
  go: linked from the system:localotelviewer viewer package
  ui: a committed build of the upstream React component, embedded so that data:release-artifact stays a pure-Go cross-compile with no Node toolchain
  boundary: host-only tooling under decision:host-tools-target-runtime, never linked into an application binary
security:
  - loopback binding only, so telemetry never leaves the machine
  - plaintext OTLP stays inside the flow:telemetry-export local boundary, so policy:outbound-transport-security is not relaxed
  - data:log-attribute and policy:query-log-safety already bound what enters a record, and the viewer adds no new source
  - the viewer holds no credential and serves no application route
acceptance:
  - pw dev with no observability configuration serves the viewer on a free loopback port and prints its URL
  - the process api:cli-dev starts receives the resolved endpoint, the protocol, and the project name in its environment
  - an OTLP export posted to the injected endpoint appears in the snapshot the UI reads
  - the same listener serves the UI page
  - a record emitted through api:logger appears in the viewer and in the developer loop terminal
  - trace_id and span_id correlate a record with its span in the UI
  - requirement:query-diagnostics records reach the viewer as logs when data:query-diagnostics-config enables them
  - an application restart preserves telemetry captured before it
  - an exported endpoint suppresses the viewer and says so
  - a variable the developer exported survives injection
  - dev.otel.enabled false starts no listener and reserves no port
  - a viewer that cannot listen reports the failure and leaves the loop running
  - a binary produced by api:cli-build contains no viewer code
non_goals:
  - persistence across pw dev runs
  - any viewer surface in api:cli-build, api:test-run, or a deployed environment
  - metric instrumentation, which requirement:contrib-otel excludes
  - search, sampling, or filtering beyond what system:localotelviewer provides
  - opening a browser, because the loop prints one stable URL and restarts must not reopen it
  - receiving telemetry from a process api:cli-dev did not start
```
