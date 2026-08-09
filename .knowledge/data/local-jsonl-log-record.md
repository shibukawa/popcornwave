---
id: data:local-jsonl-log-record
type: data
title: Local JSONL Log Record
---
One line in a requirement:local-jsonl-log-capture file is the structured form of one api:logger record, not a transcription of terminal text.

```yaml
encoding: UTF-8 JSON Lines; exactly one complete JSON object and newline per record
stable_fields:
  timestamp: RFC 3339 timestamp with subsecond precision
  severity: trace, debug, info, warn, or error
  message: string
  service_name: string
optional_correlation:
  trace_id: string
  span_id: string
  trace_flags: integer
attributes:
  source: data:log-attribute
  shape: top-level typed fields after stable and reserved fields
rules:
  - preserve string, boolean, integer, and floating-point attribute types
  - apply data:log-attribute collision and redaction rules before persistence
  - encode no terminal prefixes, ANSI control sequences, progress output, or non-api:logger child-process output
  - finish a record before exposing its newline, so a concurrently reading DuckDB query sees only complete rows
compatibility:
  - additive fields are allowed
  - stable field meanings and types do not change within a major release
  - readers spanning files use name-based schema union because application attributes vary
```
