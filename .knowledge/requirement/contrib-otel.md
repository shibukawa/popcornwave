---
id: requirement:contrib-otel
type: requirement
title: Minimal OpenTelemetry Trace and Log
---
contrib/otel provides interoperable trace propagation, manual spans, correlated structured logs, and bounded export without metrics or runtime instrumentation magic.

```yaml
packages:
  trace: contrib/otel/trace
  log: contrib/otel/log
  propagation: contrib/otel/propagation
  exporter: contrib/otel/exporter/otlphttp
trace_api:
  - Provider.Tracer(name)
  - Tracer.Start(context, name, options) returns context and Span
  - Span.SetAttributes
  - Span.RecordError
  - Span.SetStatus
  - Span.End
  - SpanContextFromContext
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
propagation:
  required: W3C traceparent and tracestate HTTP extract and inject
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
  - metrics
  - baggage
  - gRPC exporter
  - stdout exporter or failure fallback
  - auto-instrumentation
  - full API compatibility with go.opentelemetry.io/otel
standards:
  trace_context: https://www.w3.org/TR/trace-context/
  trace_api: https://opentelemetry.io/docs/specs/otel/trace/api/
  log_model: https://opentelemetry.io/docs/specs/otel/logs/data-model/
  otlp: https://opentelemetry.io/docs/specs/otlp/
```
