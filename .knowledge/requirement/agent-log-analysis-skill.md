---
id: requirement:agent-log-analysis-skill
type: requirement
title: Agent Log Analysis Skill
---
The bundled Popcorn Wave agent skill teaches an AI coding agent to inspect requirement:local-jsonl-log-capture files and produce safe DuckDB queries from a developer's natural-language question.

```yaml
canonical_skill: decision:canonical-popcornwave-skill-source
entry_point:
  description_triggers:
    - analyze, inspect, summarize, or debug pw dev logs
    - answer a question from .log JSONL files
    - create or run a DuckDB query against local logs
  navigation: SKILL.md links one focused telemetry and log-analysis reference
reference:
  location: references/telemetry.md within the Popcorn Wave skill
  covers:
    - requirement:local-jsonl-log-capture location, invocation-file lifetime, and exclusions
    - data:local-jsonl-log-record stable fields, typed custom attributes, and correlation fields
    - requirement:telemetry-architecture-guide as the human-facing explanation
workflow:
  - locate popcornwave.toml and resolve dev.logs.directory from data:project-config
  - enumerate matching closed and active .jsonl files without changing them
  - inspect inferred names and types before assuming application-specific attributes
  - translate the user's question into DuckDB SQL using read_ndjson_auto and union_by_name true
  - show the SQL before or with the result so the analysis is reproducible
  - execute only when DuckDB is available and the user asked for analysis rather than SQL text alone
  - summarize the answer, time range, files read, filters, and any schema or sampling limitation
query_policy:
  default: read-only SELECT, WITH, DESCRIBE, SUMMARIZE, or EXPLAIN over .log JSONL
  multiple_files: use a glob or explicit file list with union_by_name true
  correlation: use trace_id and span_id when the question crosses records or asks about one request
  active_file: tolerate a concurrently appended file because data:local-jsonl-log-record exposes only complete newline-terminated records
  no_data: report that no matching records exist; do not fabricate columns or results
  mutation: never delete, truncate, rewrite, compact, or move logs as part of analysis
security:
  - do not print secrets or personal values merely because a custom attribute contains them
  - prefer aggregates and selected columns over SELECT star in user-facing results
  - call out policy:query-log-safety when query bind values may be present
```

```yaml
acceptance:
  - a fresh api:cli-init project that includes the skill can answer a natural-language question about .log files without first reading the website
  - the agent discovers custom attribute columns rather than guessing them
  - queries spanning invocation files set union_by_name true
  - a request for errors in one trace filters by trace_id and orders by timestamp
  - generated queries leave every source file byte-identical
  - absence of DuckDB still yields a useful SQL query and installation-neutral next step
```
