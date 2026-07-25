---
id: data:observability-runtime-config
type: data
title: Observability Runtime Config
---
The `observability` binding configures the Popcorn Wave logger, severity filtering, trace creation, and optional requirement:contrib-otel export as one policy.

```yaml
registration: automatically registered by pw
logger_api: api:logger
emission: policy:log-emission
fields:
  minimum_level: trace, debug, info, warn, error, or off
  stdout_format: json or plaintext
  service_name: string
  resource_attributes: data:log-attribute list
  otel:
    enabled: bool
    endpoint: URL
    headers: secret-bearing string map
    request_timeout: duration
    queue_size: integer
    max_export_size: integer
    flush_interval: duration
routing:
  otel_enabled: logs and traces use requirement:contrib-otel processors and OTLP export
  otel_disabled: logs use stdout_format and trace export is disabled
rules:
  - filter records below minimum_level before formatting, allocation-heavy encoding, or export
  - stdout is not an automatic fallback after an OTLP export failure
  - copy OTLP headers and never log secret values
  - validate endpoint, queue bounds, batch bounds, durations, and level at startup
  - plaintext and JSON preserve the same message, severity, attributes, and available trace correlation
```
