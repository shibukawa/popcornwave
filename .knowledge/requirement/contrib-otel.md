---
id: requirement:contrib-otel
type: requirement
title: Minimal OpenTelemetry Trace and Log
---
contrib/otel provides interoperable trace propagation, manual spans, correlated structured logs, and bounded export without runtime instrumentation magic.

```yaml
packages:
  trace: contrib/otel/trace
  log: contrib/otel/log
  propagation: contrib/otel/propagation
  outbound_http: contrib/otel/otelhttp
  exporter: contrib/otel/exporter/otlphttp
trace_api:
  - Provider.Tracer(name)
  - Tracer.Start(context, name, options) returns context and Span
  - Span.SetAttributes
  - Span.RecordError
  - Span.SetStatus
  - Span.End
  - Span.Parent and Span.Root, a pointer chain fixed at Start so ancestor access walks no context, per requirement:context-lookup-performance; nil parent for a root or a remote extracted parent
  - SpanContextFromContext
  - SpanFromContext returning the innermost local span
log_api:
  - Logger.Emit(context, Record)
  - severity, body, timestamp, attributes, event name
  - automatic trace_id, span_id, and trace_flags correlation
attributes:
  supported:
    - string
    - bool
    - int64
    - float64
planned:
  metric_api: requirement:framework-metrics, which adds counter, up_down_counter, histogram, and observable instruments plus a /v1/metrics path, and whose inventory is data:framework-metric-set
  sampler: requirement:trace-head-sampling, which moves the record-everything rule of the trace package into a configured decision at the root span
  separation: decision:metrics-are-not-sampled, which keeps the two additions from being one
propagation:
  required: W3C traceparent and tracestate HTTP extract and inject
  extract: the server middleware, on every transport, sharing one validator per decision:propagation-header-access
  inject: whatever opened the client span, per decision:outbound-trace-propagation
  never_injected_for: the exporter's own client, which would make exporting a span open a span
export:
  - flow:telemetry-export
  - synchronous processor for tests
  - bounded batch processor for production
otlp_http:
  encoding: OTLP JSON Protobuf mapping
  traces_path: /v1/traces
  logs_path: /v1/logs
  configuration:
    - base endpoint or per-signal endpoint
    - injected http.Client
    - copied static headers
    - request timeout
non_goals:
  - baggage
  - gRPC exporter
  - stdout exporter or failure fallback
  - auto-instrumentation
  - tail sampling, which requirement:trace-head-sampling leaves to a collector and which is therefore unavailable on the direct route of flow:telemetry-export
  - full API compatibility with go.opentelemetry.io/otel
standards:
  trace_context: https://www.w3.org/TR/trace-context/
  trace_api: https://opentelemetry.io/docs/specs/otel/trace/api/
  log_model: https://opentelemetry.io/docs/specs/otel/logs/data-model/
  otlp: https://opentelemetry.io/docs/specs/otlp/
```
