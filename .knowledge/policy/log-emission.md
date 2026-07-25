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
json:
  - one record per line
  - stable top-level reserved field names
  - scalar attributes remain typed
plaintext:
  - one record per line
  - escape control characters and line breaks in message and values
failure:
  stdout: do not panic on write failure; expose bounded internal diagnostics where possible
  otel: bounded processor and flow:telemetry-export handle backpressure and export failure
security:
  - never emit observability header values or other configured secrets
  - avoid high-cardinality raw inputs in framework-generated records
```
