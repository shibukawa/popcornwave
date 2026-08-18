---
id: data:observability-runtime-config
type: data
title: Observability Runtime Config
---
The `observability` binding configures the Popcorn Web logger, severity filtering, trace creation, and optional requirement:contrib-otel export as one policy.

```yaml
registration: automatically registered by pw
logger_api: api:logger
emission: policy:log-emission
fields:
  minimum_level: trace, debug, info, warn, error, or off
  stdout_format: json or plaintext, defaulting by data:runtime-environment; dev takes plaintext because a terminal reads the slog text encoding and not a JSON object per line, and every other environment takes json for the log pipeline that consumes it
  service_name: string
  boot_log: auto, tree, record, or off; selects policy:startup-summary output
  resource_attributes: data:log-attribute list
  query: data:query-diagnostics-config sub-binding for requirement:query-diagnostics
  trace:
    purpose: selects data:framework-span-set, the spans the framework opens inside a request
    enabled: auto, on, or off; auto follows whether traces are exported, because a span nothing exports is pure cost, and on also installs the request root span so a project holding its own provider gets a complete tree
    render: bool, default true; the response span with the initial build inside it
    boundary: bool, default true, depending on render; one span per settled await boundary and per live delivery
    database: bool, default true; a client span per executed statement
    statement: bool, default true, depending on database; the statement text on that span, bounded by query.max_sql_length
    sampler: requirement:trace-head-sampling; one of the OTEL_TRACES_SAMPLER names with its argument, defaulting by data:runtime-environment per decision:sampling-default-follows-the-environment — parentbased_always_on in dev and parentbased_traceidratio at 0.1 everywhere else, the same shape stdout_format already uses for its own environment-selected default
  metrics:
    purpose: selects data:framework-metric-set, specified by requirement:framework-metrics
    enabled: auto, on, or off, resolving the way trace.enabled does; independent of trace, because decision:metrics-are-not-sampled makes counting survive a sampler that records almost nothing
    groups: one key per group of the inventory — http, db, runtime, render, cache — so a deployment already collecting the runtime group from its own agent declines it here
    interval: the reader period of flow:metric-collection, which is how coarse every chart becomes and is separate from otel.flush_interval
    temporality: delta or cumulative, per the loss section of flow:metric-collection
  otel:
    enabled: bool, default false; the parent every other otel key depends on, and derived true from an endpoint at any source
    endpoint: URL, default empty
    headers: secret-bearing key=value list, default empty
    request_timeout: duration, default 10s
    queue_size: integer, default 2048
    max_export_size: integer, default 512
    flush_interval: duration, default 5s
standard_environment:
  purpose: the OTLP variables bind to the same fields, so requirement:dev-telemetry-viewer injection needs no configuration file
  bindings:
    OTEL_SERVICE_NAME: service_name
    OTEL_EXPORTER_OTLP_ENDPOINT: otel.endpoint
    OTEL_EXPORTER_OTLP_HEADERS: otel.headers
    OTEL_TRACES_SAMPLER: trace.sampler
    OTEL_TRACES_SAMPLER_ARG: the sampler argument, which is the ratio for either traceidratio form
  excluded:
    OTEL_EXPORTER_OTLP_TIMEOUT: it counts milliseconds, while every other duration here is a Go duration string
  remaining_keys: bound to generated names, so otel.flush_interval reads OBSERVABILITY_OTEL_FLUSH_INTERVAL
routing:
  otel_enabled: logs and traces use requirement:contrib-otel processors and OTLP export, and metrics use the reader of flow:metric-collection over the same exporter
  otel_disabled: logs use stdout_format, trace and metric export are disabled, and both enabled keys auto resolve to off with it
  development_viewer: requirement:dev-telemetry-viewer injects otel enabled and endpoint into the process api:cli-dev starts, and emits to stdout as well
rules:
  - filter records below minimum_level before formatting, allocation-heavy encoding, or export
  - stdout is not an automatic fallback after an OTLP export failure
  - copy OTLP headers and never log secret values
  - validate endpoint, queue bounds, batch bounds, durations, level, and stdout_format at startup
  - a configured endpoint that cannot be used fails startup rather than degrading silently
  - service_name falls back to the executable name when nothing sets it
  - defaults restate the bounds the exporter and batch processors apply to a zero value, so a scaffolded file states what the process will do
  - a default selected by data:runtime-environment is reported with that provenance, because a sampler defaulting to a ratio and a tracer that stopped working produce the same empty trace list
  - policy:startup-summary reports every resolved value with its provenance, so an injected endpoint is visible as env
  - every otel key below enabled declares it as its dependon parent, so a run that exports nothing reports one line instead of seven
  - an endpoint from any source derives enabled true before the summary is captured, recorded at the place the endpoint came from
  - the derivation is why naming only an endpoint neither hides the address nor describes a process that does not exist
  - plaintext and JSON preserve the same message, severity, attributes, and available trace correlation
  - a trace setting that opens nothing resolves to no policy at all, so the cost of the feature off is one nil comparison rather than several false branches
  - bind values never reach a span whatever trace.statement says; policy:query-log-safety keeps row data on the record the span id correlates
```
