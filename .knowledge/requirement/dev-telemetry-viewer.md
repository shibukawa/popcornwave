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
pane_of: requirement:dev-console
responsibility:
  holds: everything the developer loop shows about what the application did, which is traces, api:logger records, requirement:query-diagnostics records, and the per-request timing a span tree already carries
  consequence: requirement:dev-console adds no log pane and no request profiler, because a second reader of the same records would be a second thing to keep correct
  instrumentation: a runtime fact a pane wants is added as a span or an attribute here, per policy:dev-console-boundary, rather than as an endpoint in the application
signals:
  traces: requirement:contrib-otel spans delivered by flow:telemetry-export, which for a pw application is data:framework-span-set — the injected endpoint turns observability.trace auto on, so the render tree and the statements are there with no configuration
  logs: api:logger records routed by policy:log-emission
  metrics: the receiver accepts /v1/metrics, but requirement:contrib-otel emits none, so the view stays empty unless the application exports its own
  process: the viewer samples cpu, memory, threads, open files, and io of the process api:cli-dev started
listener:
  split: the OTLP receiver keeps a listener of its own; the UI moves onto the requirement:dev-console listener per decision:dev-console-consolidation
  reason: the two have opposite port needs, since the receiver is an address a machine is handed and the console is one a person returns to
  receiver:
    bind: loopback only
    port: dev.otel.port, defaulting to 0 so the operating system selects a free one
    why_reserved: the endpoint is injected rather than written down, so no number has to be agreed in advance and concurrent projects never collide
    pin: a fixed port is set only when an external tool must find the receiver
    collision: a bound port fails the viewer, reports the address, and leaves the developer loop running
  ui:
    port: dev.console.port
    url: reached from the requirement:dev-console index rather than printed as a URL of its own
    mount:
      form: the whole handler under the pane prefix, through http.StripPrefix
      shared_store: receiver and pane are two mounts of one handler, so telemetry received at either address is read at both
      no_path_list: the pane names no path; the UI resolves its API against the served document and its assets relatively, so the page, the snapshot API, and the OTLP paths follow the mount together
      displayed_endpoint: the page shows its own mount base, which an exporter appends the OTLP paths to and this mount serves, so the address it offers to copy is a working one
      why_not_enumerated: the OTLP paths are specification defaults rather than the viewer's to publish, so a host that copied them would be duplicating somebody else's constants and would miss a path added later
injection:
  mechanism: environment variables on the application process, which outrank TOML in data:loaded-configuration precedence
  variables:
    - OTEL_EXPORTER_OTLP_ENDPOINT set to the resolved viewer URL
    - OTEL_EXPORTER_OTLP_PROTOCOL set to http/protobuf
    - OTEL_SERVICE_NAME set to data:project-config project.name
  naming: OTLP conventions rather than pw-specific names, so any exporter finds them
  enable_switch: not injected; data:observability-runtime-config derives it from the endpoint, which keeps every dependent key in policy:startup-summary
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
  retention: viewer state is bounded in memory by dev.otel.max and discarded at shutdown; requirement:local-jsonl-log-capture separately persists log records only
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
  - pw dev with no observability configuration receives on a free loopback port and reaches the UI from the requirement:dev-console index
  - the process api:cli-dev starts receives the resolved endpoint, the protocol, and the project name in its environment
  - an OTLP export posted to the injected endpoint appears in the snapshot the UI reads
  - the receiver port and the console port are independent, and moving one does not move the other
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
  - persistence of the viewer snapshot, traces, metrics, or process samples across pw dev runs; requirement:local-jsonl-log-capture persists logs separately
  - any viewer surface in api:cli-build, api:test-run, or a deployed environment
  - metric instrumentation, which requirement:contrib-otel excludes
  - search, sampling, or filtering beyond what system:localotelviewer provides
  - opening a browser, because the loop prints one stable URL and restarts must not reopen it
  - receiving telemetry from a process api:cli-dev did not start
```
