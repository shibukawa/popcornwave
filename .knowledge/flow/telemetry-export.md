---
id: flow:telemetry-export
type: flow
title: Telemetry Export Flow
---
Telemetry export batches completed spans and log records within bounded memory and never blocks application shutdown indefinitely.

```yaml
flow:
  trigger: requirement:contrib-otel emits a completed span or log record
  steps:
    - id: enqueue
      action: append to a bounded queue
      failure: increment dropped-record count without blocking
    - id: batch
      action: flush on maximum batch size or interval
    - id: encode
      action: encode OTLP JSON Protobuf mapping
    - id: send
      action: POST OTLP/HTTP traces to /v1/traces and logs to /v1/logs with the configured http.Client
    - id: retry
      action: retry transient failures with bounded exponential backoff and jitter
      exhausted: account for dropped records and expose exporter error without stdout fallback
  shutdown:
    - reject new records after closing starts
    - flush within context deadline
    - return final exporter error
security:
  - policy:outbound-transport-security applies; verified HTTPS is required outside the local proxy boundary
  - configurable headers are copied and secrets are not logged
  - response bodies and diagnostic text are size bounded
```
