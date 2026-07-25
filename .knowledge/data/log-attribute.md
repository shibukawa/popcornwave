---
id: data:log-attribute
type: data
title: Log Attribute
---
Logger attributes reuse the scalar attribute contract shared with requirement:contrib-otel traces and logs.

```yaml
shape:
  key: non-empty string
  value: string, bool, int64, or float64
constructors:
  - String(key, value)
  - Bool(key, value)
  - Int64(key, value)
  - Float64(key, value)
reserved_record_fields:
  - timestamp
  - severity
  - message
  - trace_id
  - span_id
  - trace_flags
rules:
  - user attributes cannot replace reserved record fields
  - later duplicate user keys replace earlier values deterministically
  - framework code never adds credentials, tokens, cookies, or raw personal inputs by default
  - application code owns classification and redaction of its custom attributes
```
