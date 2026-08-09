# Telemetry and local log analysis

Use this reference when a user asks what happened during `pw dev`, requests a
log investigation, or wants a DuckDB query for Popcorn Wave logs.

## Locate the records

`pw dev` keeps application logs readable in the terminal and also writes the
same structured records as newline-delimited JSON. The default glob is
`.log/*.jsonl`. A project can change or disable it in `popcornwave.toml`:

```toml
[dev.logs]
enabled = true
directory = ".log"
```

Resolve `directory` relative to the project root. Do not assume `.log` when the
key is present. Each `pw dev` invocation owns one file; application rebuilds
continue appending to that file. Empty runs leave no file.

Enumerate the matching `.jsonl` files before querying and state whether the
selection includes the active run. The files contain application logger records,
not traces, service stdout, or `pw` progress lines. For the human-facing model,
see the [Telemetry guide](https://shibukawa.github.io/popcornwave/guides/architecture/telemetry/).

## Query with DuckDB

Prefer queries that read the JSONL directly. Do not create a database, rewrite
records, or delete log files unless the user explicitly asks.

```sql
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true)
ORDER BY timestamp DESC
LIMIT 100;
```

`union_by_name = true` accommodates optional and evolving custom fields. Start
by inspecting the inferred columns when a query depends on application-specific
attributes:

```sql
DESCRIBE SELECT *
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true);
```

Stable fields are `timestamp`, `severity`, `message`, and `service_name`.
Correlated records may add `trace_id`, `span_id`, and numeric `trace_flags`.
Custom attributes remain typed top-level JSON values, so inspect before casting.

Useful starting points:

```sql
-- Recent warnings and errors.
SELECT timestamp, severity, service_name, message, trace_id
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true)
WHERE lower(severity) IN ('warn', 'error')
ORDER BY timestamp DESC
LIMIT 100;

-- Repeated messages by service and severity.
SELECT service_name, severity, message, count(*) AS occurrences
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true)
GROUP BY ALL
ORDER BY occurrences DESC
LIMIT 50;

-- One trace in event order. Bind or safely quote the requested ID.
SELECT timestamp, severity, message, span_id
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true)
WHERE trace_id = $trace_id
ORDER BY timestamp;
```

Use only read-only statements such as `SELECT`, `FROM`, `WITH`, `DESCRIBE`,
`SUMMARIZE`, and `EXPLAIN` during investigation. A running application may append
a new line while DuckDB scans; retry once if the active final record is
temporarily incomplete.

Show the SQL with any result so the investigation is reproducible. Execute it
only when DuckDB is available and the user asked for analysis; otherwise return
the useful SQL and an installation-neutral note that DuckDB must be supplied
externally. If no records or inferred column match, say so rather than inventing
a schema.

## Report findings

State the files or glob, time window, filters, and query behind the conclusion.
Summarize patterns instead of pasting bulk records. Treat every custom field as
potentially sensitive: redact secrets, tokens, personal data, and session values
from the response even when they were accidentally logged. Query diagnostics
can optionally record bind values, so treat SQL-related custom fields with the
same caution.
