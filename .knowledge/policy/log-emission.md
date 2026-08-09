---
id: policy:log-emission
type: policy
title: Log Emission Policy
---
Every api:logger call follows one severity, correlation, and output-selection pipeline.

```yaml
pipeline:
  - compare severity with data:observability-runtime-config minimum_level
  - merge Logger.With and call attributes using data:log-attribute rules
  - add timestamp, severity, message, and available request and trace correlation
  - route to OTLP when OpenTelemetry is enabled
  - otherwise encode one stdout JSON object or one plaintext record
routing_exception:
  scope: requirement:dev-telemetry-viewer under api:cli-dev only
  rule: emit to both OTLP and stdout, because the developer loop stream is the primary surface and the viewer is the correlated one
  unchanged: every other environment routes exclusively
development_file_sink:
  scope: requirement:local-jsonl-log-capture under api:cli-dev only
  rule: tee the structured record before stdout formatting, independently of the OTLP routing choice
  failure: a local file failure disables only that sink
json:
  - one record per line
  - stable top-level reserved field names
  - scalar attributes remain typed
plaintext:
  - the slog text encoding, one key=value record per line
  - the default under data:runtime-environment dev, including the api:cli-dev stream, because that stream is read by a person rather than by a collector
  - escape control characters and line breaks in message and values
failure:
  stdout: do not panic on write failure; expose bounded internal diagnostics where possible
  otel: bounded processor and flow:telemetry-export handle backpressure and export failure
security:
  - never emit observability header values or other configured secrets
  - avoid high-cardinality raw inputs in framework-generated records
```
