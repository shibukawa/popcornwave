---
id: requirement:telemetry-architecture-guide
type: requirement
title: Telemetry Architecture Guide
---
The documentation provides one end-to-end telemetry page at `website/src/content/docs/guides/architecture/telemetry.md` and a Japanese peer under the Architecture navigation group.

```yaml
audience: application developers instrumenting, running, debugging, or operating a Popcorn Web application
purpose:
  - explain how logs, traces, query diagnostics, correlation, sinks, and development tools fit together
  - provide the starting page that the current scattered references lack
page_rule:
  - describe the architecture once and link detailed API and configuration references
  - distinguish api:cli-dev defaults from deployed-runtime behavior
  - keep English and Japanese pages structurally equivalent
sections:
  mental_model:
    covers: api:logger, data:log-attribute, data:framework-span-set, requirement:query-diagnostics, and trace/span correlation
  emission_pipeline:
    covers: policy:log-emission and flow:telemetry-export
    contrasts: terminal plaintext, JSON stdout, local JSONL, OTLP, and the development viewer
  application_logging:
    covers: acquiring api:logger from context, levels, typed attributes, correlation, reserved fields, and safe attribute selection
  configuration:
    covers: data:observability-runtime-config, relevant environment variables, and data:project-config dev.otel and dev.logs
    links: configuration reference remains authoritative for every key and default
  development:
    covers: api:cli-dev terminal output, requirement:dev-telemetry-viewer, requirement:local-jsonl-log-capture, run-file lifetime, .gitignore, and cleanup
  duckdb:
    prerequisite: DuckDB is an optional external analysis tool, not a pw dependency
    agent_workflow: requirement:agent-log-analysis-skill turns natural-language questions into reproducible queries over the same files
    load_all: "SELECT * FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true);"
    examples:
      - filter by severity and time
      - aggregate counts by message and service_name
      - find a trace_id and order its records by timestamp
      - inspect one invocation through the virtual filename column when supported by the installed DuckDB version
    note: use name-based schema union because custom log attributes differ between records and runs
  production:
    covers: JSON stdout for a collector, OTLP routing, queue and shutdown behavior, and why local files are pw dev only
    routes: the relayed and direct routes of flow:telemetry-export as two named choices, with what each costs and the fact that tail sampling exists only on the first
    sampling: requirement:trace-head-sampling and the environment-selected default of decision:sampling-default-follows-the-environment, stated as a number a reader will find in their startup summary rather than as a key they may set
    counting_is_unaffected: requirement:framework-metrics survives any ratio, per decision:metrics-are-not-sampled, which is the paragraph that keeps a reader from raising the ratio to fix a dashboard
  safety:
    covers: secrets, personal data, query bind values, local file permissions, retention, and deletion
navigation: files are discovered automatically under Architecture; no manual sidebar entry
```

```yaml
acceptance:
  - both locales build and appear under Architecture
  - a reader can find the logger API, all observability configuration, local capture, viewer, OTLP, and DuckDB workflow from this page
  - the page points agent users to requirement:agent-log-analysis-skill rather than requiring them to author SQL by hand
  - examples use data:local-jsonl-log-record field names and execute against valid sample records
  - DuckDB examples use newline-delimited JSON input and schema union by name
  - the page does not imply that pw bundles, installs, or runs DuckDB
  - the page does not claim traces or arbitrary process output are present in .log
  - a reader can tell, from this page alone, whether their deployment is on the relayed or the direct route and what its trace sampling therefore is
  - the page states that a low sampling ratio changes no metric
```
