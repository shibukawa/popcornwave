---
title: Request Tracing
description: Read a request as a span tree, where the render branch, the initial build, each boundary that landed, and each statement that ran are separate spans.
sidebar:
  order: 5
---

A slow page has a shape, and a duration does not show it. `842ms` tells you the
request was slow; it does not tell you whether the shell took 800ms and the
async boundary 40, or the shell took 40 and one query inside a boundary took
800. Those two pages need opposite fixes, and until the framework says which one
you are looking at, the only way to find out is to add timers.

So the framework opens the spans itself. A traced request arrives as a tree:

```
GET /orders                                      842ms
└─ render stream                                 838ms
   ├─ render initial                              41ms
   │  └─ SELECT                                   33ms
   ├─ render boundary  (tb-1)                    797ms
   └─ render boundary  (tb-2)                    120ms
```

The shell was fast. One boundary held its fallback for most of a second, and it
is `tb-1` rather than `tb-2`. That is the whole diagnosis, and nothing in the
application had to be instrumented to get it.

## Turning it on

Nothing, if you are running [`pw dev`](/productivity/dev-telemetry-viewer/). The
development loop points the application at a local receiver, and the default
setting follows that:

```toml
[observability.trace]
enabled = "auto"   # on when traces are exported, off when they are not
```

`auto` is the honest default rather than a cautious one. A span costs an
allocation, a pair of timestamps, and an export, and a process with nowhere to
send it pays all three so the record can be dropped. Naming an OTLP endpoint is
what turns export on — see
[`[observability.otel]`](/reference/configuration/#observabilityotel) — and the
same act turns these spans on with it.

Set `enabled = "on"` when you hold your own tracer provider and want the tree
whatever the endpoint configuration says. Set it to `"off"` when a route is hot
enough that even a handful of spans per request is a cost you have measured and
do not want.

## The render span

Every HTML response opens one span, and its name says which of six branches the
response took. That name is worth more than it looks, because the branch is
often the surprise:

| Name | What the response was |
| --- | --- |
| `render buffered` | the whole document built before the first byte |
| `render stream` | [async rendering](/guides/cross-layer/async-rendering/): shell first, boundaries after |
| `render live` | a [live](/guides/cross-layer/live-rendering/) delivery stream, not the document |
| `render navigate` | a [navigation delta](/guides/cross-layer/partial-updates/) |
| `render redraw` | one component answering for itself |
| `render fragment` | a bare fragment for an external swap library |

A page you wrote for streaming that arrives as `render buffered` has been
classified as a crawler, or is running with `html.streaming` off. Neither is
visible in the response body — the bytes are a complete, correct document either
way — and both change the timing you are looking at. The span attributes say
which: `pw.render.bot` names the classification, and `pw.render.async` and
`pw.render.live` say what the composed chain actually contains.

The span also carries `pw.render.bytes`, the uncompressed size that reached the
client, and `pw.render.boundaries`, the number of regions that settled.

A response that consulted the [render cache](/guides/frontend/rendering-cache/)
adds `pw.render.cache_hits` and `pw.render.cache_misses`; one that reached no
store carries neither. Both halves are reported because a hit count alone cannot
separate a cache that is working from one almost nothing reaches, and nothing
else in the system reviews the TTL an author guessed. A component whose
parameters differ on every call renders exactly as an uncached one would while
still paying for a key and a buffer — this pair is what says so.

## Where the first paint ends

Inside a streaming render, one child span covers the initial build: the document
shell, the merged head, and every fallback. It ends at the flush that commits
the document, which is the moment the visitor first sees something.

That timestamp is what makes the rest of the tree readable. Each boundary span
starts there and ends when its fragment was written, so its extent is exactly
how long that fallback sat on screen. It is not how long the boundary's own work
took — a boundary waiting behind
[`html.async_concurrency`](/reference/configuration/#html) spends part of that
span queued — and the difference matters when you are deciding whether to make a
query faster or to raise a limit. The span measures the visitor's experience; a
span you open inside the work with
[`pw.StartSpan`](/reference/runtime/#tracing) measures the work.

A live response is the same idea on a longer clock. Each delivery span runs from
that boundary's previous delivery, so consecutive spans abut and the waterfall
reads as one region changing rather than as a scatter of instants. The render
span closes with `pw.live.close_reason`, which is `done` when the sources ended
and `retry` when a [subscription
bound](/guides/cross-layer/live-rendering/) closed a healthy stream.

Buffered and fragment responses open no initial-build child. There is nothing
after the initial pass on those branches, so a second span with its parent's
exact extent would say the same thing twice.

## The statements underneath

Every function generated from a `.pw.sql` file resolves its handle through one
place in the framework — the same seam [query
diagnostics](/productivity/query-diagnostics/) uses — so each statement becomes
a client span named after its keyword:

```
├─ render boundary  (tb-1)                       797ms
   └─ SELECT                                     791ms
      db.system.name    = postgresql
      db.operation.name = SELECT
      db.query.text     = SELECT * FROM orders WHERE customer = $1
      pw.db.slow        = true
```

The statement text is on the span; the bind values are not, and no setting puts
them there. A trace backend is retained longer and read more widely than an
application log, so row data stays on the query record instead — and that record
names this span rather than the request root, so the trace entry leads to the
values, the plan, and the [rerun
snippet](/productivity/query-diagnostics/#rerunning-it-by-hand). One click, not
one search.

`pw.db.slow` appears once a statement passes `observability.query.slow_threshold`,
which is where a waterfall and a log meet: the span tells you which one to open.

## What to switch off

Three keys sit under the parent, and each answers a different complaint.

```toml
[observability.trace]
enabled = "auto"
render = true      # the response span and everything inside it
boundary = true    # a span per settled boundary and per live delivery
database = true    # a client span per statement
statement = true   # the SQL text on that span
```

`boundary = false` is the one to reach for first on a page with many small
regions, or on a live screen delivering every second — a long-lived response can
otherwise produce hundreds of spans that all say the same thing. The count
survives as `pw.render.boundaries`, because it is an attribute rather than a
span.

`statement = false` keeps the timing and drops the SQL. Turn it off when your
statements are long enough that the text dominates the export, not out of
caution about the content: the text is the parameterised form your `.pw.sql`
file holds, with no values in it.

`database = false` without turning off the query log is the reverse pair, and it
is the right shape when a collector already sees your statements through a
database proxy.

A setting that opens nothing resolves to no policy at all, so switching the
parent off costs one comparison per response and per statement — no wrapped
writer, no wrapped executor, no span object anywhere.

## Where it stops

Framework spans cover framework work. The handler's own calls — an HTTP request
to another service, a cache lookup, work started with
[`pw.Go`](/guides/cross-layer/async-rendering/) before the render began — are
yours to open, with [`pw.StartSpan`](/reference/runtime/#tracing), and they nest
under whichever span is active when you call it.

Two gaps are worth knowing about rather than discovering. Statements the
framework issues for itself — session, auth, and migration SQL — go straight to
the pool and produce no span, the same exclusion the query log applies. And a
`render navigate` or `render redraw` span covers the whole call rather than the
regions inside it, because the comparison that decides which regions to send
happens below the framework's own code.

For the full key list see
[`[observability.trace]`](/reference/configuration/#observabilitytrace). For
reading the result without installing anything, see the [development telemetry
viewer](/productivity/dev-telemetry-viewer/).
