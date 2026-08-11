---
title: Telemetry
description: How Popcorn Wave connects application logs, traces, development diagnostics, local JSONL files, and DuckDB analysis.
sidebar:
  order: 7
---

Telemetry answers two different questions. Logs explain discrete decisions and
failures; traces show how one request moved through work and time. Popcorn Wave
keeps them correlated without forcing development and production to use the
same destination.

## The telemetry model

Application code writes structured records through
[`pw.Logger(ctx)`](/reference/runtime/#logging). The
context carries active trace identifiers, so a record emitted during a span can
include `trace_id`, `span_id`, and `trace_flags` automatically. Framework
diagnostics use the same logger. A message is useful to a person; typed
attributes make the same event queryable.

```go
package handlers

import (
    "net/http"

    "github.com/shibukawa/popcornwave/pw"
)

func showAccount(w http.ResponseWriter, r *http.Request) {
    accountID := r.PathValue("id")
    pw.Logger(r.Context()).Info(
        "account requested",
        pw.String("account_id", accountID),
        pw.Bool("cached", false),
    )
    w.WriteHeader(http.StatusNoContent)
}
```

Levels run from `trace` through `error`, with `off` available as a configured
floor. Use `pw.String`, `pw.Int`, `pw.Bool`, and the other scalar constructors
instead of flattening values into the message. `timestamp`, `severity`,
`message`, `service_name`, `trace_id`, `span_id`, and `trace_flags` are reserved
for the emission pipeline and cannot be replaced by application attributes.

Generated database calls can add statement and timing records through
[Query Diagnostics](/productivity/query-diagnostics/). Bind-value logging is a
separate, sensitive setting; it is not necessary for trace correlation.

Do not put passwords, tokens, session values, or unnecessary personal data in
messages or attributes. Structured storage makes accidental secrets easier to
search, not safer to keep.

## Reading a request trace

A duration says that a request was slow; a span tree says where it waited. The
framework opens the request, render, boundary, and generated-database spans, so
the common path needs no application timers:

```
GET /orders                                      842ms
└─ render stream                                 838ms
   ├─ render initial                              41ms
   │  └─ SELECT                                   33ms
   ├─ render boundary  (tb-1)                    797ms
   └─ render boundary  (tb-2)                    120ms
```

Here the shell was fast and `tb-1` kept its fallback visible for most of a
second. The development telemetry viewer shows this tree automatically. Outside
that loop, naming an OTLP endpoint enables the default `auto` policy:

```toml
[observability.trace]
enabled = "auto"   # on when traces are exported, off otherwise
render = true
boundary = true
database = true
statement = true
```

Use `enabled = "on"` with an application-owned tracer provider, or `"off"` when
you have measured the span cost on a hot route. `boundary = false` is the first
detail to remove from a page with many small or frequently delivered regions.
`statement = false` keeps database timing but omits SQL text; bind values are
never added to spans.

### The render branch

The render span name identifies the response path:

| Name | Response path |
| --- | --- |
| `render buffered` | a complete document built before the first byte |
| `render stream` | an [async-rendered](/guides/cross-layer/async-rendering/) document |
| `render live` | a [live](/guides/cross-layer/live-rendering/) delivery stream |
| `render navigate` | a [navigation delta](/guides/cross-layer/partial-updates/) |
| `render redraw` | one component answering for itself |
| `render fragment` | a fragment for an external swap library |

The span carries `pw.render.bytes` before response compression and
`pw.render.boundaries`. A response that consulted the [render
cache](/guides/frontend/rendering-cache/) also carries
`pw.render.cache_hits` and `pw.render.cache_misses`.

In a streamed document, `render initial` ends at the flush that commits the
shell and fallbacks. Each boundary span starts there and ends when its fragment
is written, so its duration is how long the visitor saw the fallback, including
time queued behind `html.async_concurrency`. Open a custom
[`pw.StartSpan`](/reference/runtime/#tracing) inside the work when you need to
separate queue time from execution time.

Generated `.pw.sql` calls appear beneath the active render or application span.
They carry the parameterized statement and standard database attributes, never
bind values. Slow statements add `pw.db.slow`; the correlated [query
diagnostic](/productivity/query-diagnostics/) contains the values, plan, and
rerun snippet under the same trace identifiers.

## Traces that cross services

Inside one process, correlation is the pair of identifiers the context already
carries. Across a service boundary there is no shared context, so the
identifiers travel on the wire instead, as the W3C Trace Context `traceparent`
and `tracestate` fields.

Reading them is automatic. A request arriving with a `traceparent` continues the
caller's trace rather than starting one, on either HTTP backend, so the root of
the tree is whichever service the request actually entered first. Writing them
takes an instrumented client, because the header has to be produced by whatever
opened the client span — the callee adopts the span the header names, and a
header written anywhere else attributes the callee's work to the wrong parent.

```go
import "github.com/shibukawa/popcornwave/contrib/otel/otelhttp"

client := otelhttp.NewClient(http.DefaultClient)
request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
if err != nil {
    return err
}
response, err := client.Do(request)
```

Pass the request context; replacing it with `context.Background()` breaks the
parent relationship. `otelhttp.NewTransport` provides the same instrumentation
for a custom client without modifying the transport you pass in.

This is what makes the `trace_id` in a local JSONL record worth carrying to the
collector. The one-trace query further down returns the records *this* service
wrote; the same ID in the collector returns the spans every other service
recorded for the same request.

One client is deliberately excluded: the one the OTLP exporter posts through.
Tracing an export makes the export open a span, whose export opens another, so
the exporter removes the instrumentation from the client it is given.

Framework spans stop at framework work. Open custom spans around cache calls or
other handler work with [`pw.StartSpan`](/reference/runtime/#tracing). Session,
authentication, and migration statements issued internally do not produce
database spans, matching their exclusion from query diagnostics.

## Development destinations

During `pw dev`, application records remain visible as readable text in the
terminal. The development telemetry viewer receives correlated logs and traces
when enabled. Separately, `pw dev` captures application log records as JSONL in
the project-local `.log` directory by default.

Each `pw dev` invocation gets one file named `pw-dev-*.jsonl`. Rebuilds and
application restarts append to that invocation's file; a later invocation uses
a new file. The directory and file are created lazily on the first record, so a
silent run leaves nothing behind. Existing files are never truncated or
automatically deleted, and new `pw init` projects ignore `.log/` in Git.
Existing projects should add `.log/` to their own `.gitignore`. Directories and
files are created with owner-only permissions, subject to operating-system rules.

```toml
[dev.logs]
enabled = true
directory = ".log"
```

The viewer has its own independent project setting:

```toml
[dev.otel]
enabled = true
```

`directory` must be a relative path inside the project. Set `enabled = false`
to keep terminal and configured OTLP output without local files. A filesystem
error disables only local capture and prints one diagnostic; it does not stop
the application.

Every JSON line has stable `timestamp`, `severity`, `message`, and
`service_name` fields. Correlated records can add `trace_id`, `span_id`, and
numeric `trace_flags`. Application attributes remain typed top-level fields,
which keeps numbers and booleans useful in queries.

## Query JSONL with DuckDB

DuckDB is an optional external tool; `pw` neither bundles, installs, nor runs it.
It can query all invocations without importing them into a database. Run it
from the project root, or adjust the glob to the configured directory. The
Popcorn Wave agent skill includes this schema and can turn a question such as
“show repeated errors from the last hour” into a query.

```sql
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true)
ORDER BY timestamp DESC
LIMIT 100;
```

`union_by_name = true` lets files contain different optional application
attributes. Inspect inferred types before filtering an unfamiliar field:

```sql
DESCRIBE SELECT *
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true);
```

Recent warnings and errors:

```sql
SELECT timestamp, severity, service_name, message, trace_id
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true)
WHERE lower(severity) IN ('warn', 'error')
  AND timestamp >= now() - INTERVAL '1 hour'
ORDER BY timestamp DESC
LIMIT 100;
```

Repeated events:

```sql
SELECT service_name, severity, message, count(*) AS occurrences
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true)
GROUP BY ALL
ORDER BY occurrences DESC
LIMIT 50;
```

One trace in event order:

```sql
SELECT timestamp, severity, message, span_id
FROM read_ndjson_auto('.log/*.jsonl', union_by_name = true)
WHERE trace_id = 'REPLACE_WITH_TRACE_ID'
ORDER BY timestamp;
```

To isolate one invocation, DuckDB versions supporting the JSON reader's
`filename` option can expose the source path as a virtual column:

```sql
SELECT filename, timestamp, severity, message
FROM read_ndjson_auto(
    '.log/*.jsonl',
    union_by_name = true,
    filename = true
)
WHERE filename = '.log/REPLACE_WITH_RUN_FILE.jsonl'
ORDER BY timestamp;
```

Keep exploratory analysis read-only. Bind or carefully quote user-provided
values instead of concatenating SQL. A running application can append while a
scan is in progress; retry if the newest record is temporarily incomplete.

## Production destinations

Outside `pw dev`, Popcorn Wave does not create local log files. Production logs
are structured JSON on standard output for the platform's log collector, and
OTLP export goes to the configured collector. This keeps file ownership,
rotation, retention, access control, and deletion with the deployment platform
rather than an application container.

Configure levels, `stdout_format`, service identity, resource attributes, and
OTLP endpoint/headers through TOML or the corresponding `OTEL_*` environment
variables in
[Application Configuration](/reference/configuration/#observability). The local
capture switch belongs to `popcornwave.toml`, because it controls the developer
process rather than the deployed application configuration.

OTLP uses a bounded queue so request handling does not wait for a collector; a
full queue drops records. It exports partial batches on the configured flush
interval and performs a final bounded shutdown flush. Those queue, batch,
request-timeout, and shutdown-timeout settings are documented in the same
configuration reference.

## When not to use local capture

Do not use `.log` as deployed storage, an audit trail, or a substitute for a
collector. It has no rotation, retention enforcement, shipping, or access-control
workflow. Likewise, use traces rather than adding start/finish log pairs when
the question is only where a request spent time.

## Operational checklist

- Log stable event names and typed attributes; avoid formatting data into the message.
- Pass request contexts to `pw.Logger` so records retain trace correlation.
- Keep `.log/` uncommitted, delete old files according to local policy, and never treat it as production storage.
- Share queries and summaries, not raw files that may contain sensitive fields.
- Use the telemetry viewer for request shape and timing; use DuckDB when the question spans many records or runs.

See [`pw dev`](/pw/project/dev/#telemetry-viewer) for the complete development
loop and [Development Telemetry Viewer](/productivity/dev-telemetry-viewer/) for
the interactive trace interface.
