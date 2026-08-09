---
id: requirement:local-jsonl-log-capture
type: requirement
title: Local JSONL Log Capture
---
api:cli-dev persists structured api:logger records under `.log` so a developer can analyze a run locally without replacing the readable terminal stream or installing a collector.

```yaml
scope: api:cli-dev application log records only
default: enabled
configuration: data:project-config dev.logs
source:
  - api:cli-dev selects the invocation file and injects its path only into each application process it starts
  - persist from the api:logger structured pipeline before stdout formatting and independently of OTLP or requirement:dev-telemetry-viewer
  - never parse policy:log-emission plaintext stdout
destination:
  directory: .log relative to the directory containing popcornwave.toml
  file: one collision-resistant .jsonl file per pw dev invocation
  lifetime: select before the application starts; reopen for append across every rebuild and application restart; close when each application process stops
  creation: create the directory and file lazily on the first record
  existing_files: never truncate or append to a file from an earlier invocation
record: data:local-jsonl-log-record
terminal:
  rule: policy:log-emission stdout behavior remains unchanged
  consequence: each application record remains visible as plaintext in the pw dev console and is also queryable on disk
failure:
  startup: an invalid or unwritable configured directory reports the path and continues pw dev with terminal logging
  write: report a bounded diagnostic once, disable file capture for the run, and never stop the application
retention:
  automatic_deletion: none in the first release
  ownership: developers may remove closed files; the active file is owned by pw dev until shutdown
repository_hygiene:
  - api:cli-init scaffolds `.log/` in .gitignore
  - documentation tells existing projects to ignore `.log/`
  - no log file is a build input, generated source, or release artifact
security:
  - local persistence does not weaken data:log-attribute or policy:query-log-safety
  - documentation warns that custom attributes and enabled query bind values can contain sensitive data
  - create directories and files with owner-only permissions
```

```yaml
acceptance:
  - a pw dev run creates no file before its first api:logger record
  - the first record creates .log and one .jsonl file containing one valid data:local-jsonl-log-record line
  - a rebuild keeps writing to the same invocation file
  - a second pw dev invocation creates a different file and preserves the first
  - stdout still receives the readable record while capture is enabled
  - dev.logs.enabled false creates no directory or file and changes no stdout behavior
  - capture still works when dev.otel.enabled is false or a developer-supplied OTLP endpoint suppresses the viewer
  - child service stdout and pw progress lines never enter the JSONL file
  - a file failure neither terminates nor restarts the application
non_goals:
  - deployed runtime log files
  - trace or metric persistence
  - indexing, retention deletion, compression, or shipping
  - parsing arbitrary stdout or stderr
```
