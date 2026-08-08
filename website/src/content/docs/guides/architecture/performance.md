---
title: Performance
description: A scale for request costs and the settings to check before a production release.
sidebar:
  order: 5
---

Popcorn Wave handles sessions, CSRF, security headers, request IDs, and other
request bookkeeping by default. That work does not make the framework inherently
slower: in the todo benchmark, Popcorn Wave completed a request with less CPU time
than the hand-written `net/http` service.

That result can change with the application and the machine. Treat the numbers
below as a scale for deciding where to investigate, not as a performance guarantee.

## Cost by layer

These results come from
[`examples/todo`](https://github.com/shibukawa/popcornwave/tree/main/examples/todo)
under 20 concurrent clients. They report CPU time attributed to each request on
Go 1.26.5 and an Apple M3, with PostgreSQL 17 in Docker on the same machine.

| Per request | CPU time |
| --- | --- |
| Whole middleware chain | 2.7 µs |
| └ CSRF check | 2.2 µs |
| One `SELECT` returning 50 rows | 37 µs |
| Encode and write a JSON response | 30 µs |
| Render and write an HTML response | 95 µs |
| Whole request, Popcorn Wave | 203 µs |
| Whole request, `net/http` comparison | 221 µs |

The JSON and HTML rows describe different responses, so the table is not a sum.
Its useful signal is the order of magnitude. The middleware chain takes a few
microseconds, while a simple query takes tens and HTML rendering takes more. A
database call across a real network may take hundreds of microseconds or several
milliseconds.

Measure which layer dominates your application first. Removing a few microseconds
of middleware only matters after that layer has proved to be the bottleneck.

## Settings to check before a production release

A development setting that reaches production can cost more than a small code
optimization can recover.

### Environment and logging

`config.dev.toml` enables debug output and SQL logging. Load a production
configuration with `APP_ENV=prod` or the corresponding environment name for load
tests and production, and verify that query logging is off. If you enable detailed
logging during an investigation, turn it off again when the investigation ends.

[Query diagnostics](/productivity/query-diagnostics/) explains how to investigate
slow queries without leaving all production SQL logging enabled.

### Session backend

The `cookie` backend needs no external store, and its cryptographic work costs
about 0.5 µs per request for a typical session. Its real scaling constraint is the
wire: the response carries the whole session record, and the browser returns that
record on the next request. Larger sessions therefore mean larger request and
response headers.

The `rdb`, `redis`, `dynamo`, and `firestore` backends leave only a small identifier
in the browser. They reduce the bytes on the wire, but add a storage access to each
request.

```toml
[session]
enabled = true
backend = "rdb" # or redis, dynamo, firestore
```

Use `cookie` for a small session when avoiding another dependency matters. Choose
a server-side backend when revocation, persistence, size, or network transfer
belongs on the server. Operational requirements should narrow the choice first;
then measure with the expected session size and storage latency. `dev-volatile`
loses every session on restart and is restricted to development.

See [Sessions](/guides/backend/sessions/) for the persistence and revocation
tradeoffs.

### Database connections

A production configuration can contain more than one connection. A common layout
uses one group for writes and another for reads; multiple connections in the same
reader group are selected round-robin.

```toml
[middleware.rdb]
enabled = true
default_group = "reader"
write_group = "writer"
migration_group = "writer"

[[middleware.rdb.connections]]
group = "writer"
dsn = "postgres://app:${DB_PASSWORD}@writer.example/app"
max_open_conns = 20

[[middleware.rdb.connections]]
group = "reader"
dsn = "postgres://app:${DB_PASSWORD}@reader-1.example/app"
readonly = true
max_open_conns = 20

[[middleware.rdb.connections]]
group = "reader"
dsn = "postgres://app:${DB_PASSWORD}@reader-2.example/app"
readonly = true
max_open_conns = 20
```

With this configuration, an unpinned query uses `reader`. Handlers that only read
can keep using that default, but Popcorn Wave does not infer a destination from the
SQL text. Write paths therefore need an explicit code change.

```go
// One write.
user, err := queries.CreateUser(pw.SelectDB(ctx, "writer"), name)

// A write transaction.
err := pw.Transaction(pw.SelectDB(ctx, "writer"), func(ctx context.Context) error {
	return queries.RecordAudit(ctx, "user.created")
})
```

Reads that must observe a preceding write should also use `writer` rather than wait
for replica convergence. [Relational databases](/guides/storage/rdb/) describes
connection groups and transaction behavior in detail.

Pool limits apply to each connection. Before increasing `max_open_conns`, add the
limits across all configured connections, multiply that sum by the number of
application instances, and verify that the database can accept the resulting total.

### Compression and CSRF

Response compression is off by default. Enable it only when a CDN or reverse proxy
is not already doing that work; see
[Response compression](/guides/frontend/compression/).

CSRF is the most visible item in the middleware measurement, but it still costs
only 2.2 µs in this benchmark. Exclude only APIs that use bearer authentication and
cannot be called from a browser with cookies. Removing protection from a wider path
is not a useful performance trade.

```toml
[security.csrf]
enabled = true
include = ["/**"]
exclude = ["/api/**"]
```

## Measure the application

Use production-equivalent settings and compare the same features, response, and
data. Match the instrument to the decision: latency for user wait time, throughput
for capacity, and a CPU profile for work performed in code.

Once a profile identifies a candidate, replace that layer and measure again. If the
HTTP stack itself proves to be the limit, [Why Popcorn Wave?](/start/why-popcorn-wave/#when-the-http-stack-is-the-measured-bottleneck)
describes the boundary and the alternatives.
