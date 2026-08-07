---
title: Performance
description: What each part of a request costs in microseconds, why a simple benchmark makes a working framework look slow, and the five settings worth tuning in production.
sidebar:
  order: 5
---

Benchmark a Popcorn Wave service against a hand-written `net/http` one and the
result depends almost entirely on one setting. On the todo pair in
[`examples/todo`](https://github.com/shibukawa/popcornwave/tree/main/examples/todo)
— one PostgreSQL table, same five routes — the framework is 8% behind with a
cookie session and 9% *ahead* with a server-side one, on the same code.

| Session backend | Throughput vs baseline | HTML p95 | `Set-Cookie` per response |
| --- | --- | --- | --- |
| `cookie` | −8% | 2.90–3.21 ms | 631 B |
| `dev-volatile` | +9% | 2.46–2.66 ms | 299 B |

Baseline HTML p95 was 3.15–3.39 ms, so the framework is quicker to first byte in
both configurations, and about 22% quicker with a server-side session.

Medians of five alternating passes on an otherwise idle machine. Read the
ratios rather than the absolute rates, which belong to the machine that produced
them: a session on a busy machine moved the baseline from ~14,900 to ~11,600
requests per second and left both ratios recognisable.

This page is in microseconds rather than percentages wherever it can be,
because a percentage is only true for the workload it was measured on. "The
session costs 0.8% of CPU" becomes a different number the moment you add a
second query, and it tells you nothing you can add to your own request budget.
Microseconds compose.

## What each part of a request costs

Measured on the todo service under 20 concurrent clients, CPU time attributed
per request. Go 1.26.5, Apple M3, PostgreSQL 17 in Docker on the same machine.

| Per request | CPU |
| --- | --- |
| **Whole middleware chain** | **2.7 µs** |
| — of which the CSRF check | 2.2 µs |
| One `SELECT` returning 50 rows | 37 µs |
| Render and write the HTML page | 95 µs |
| Encode and write the JSON response | 30 µs |
| **Whole request, Popcorn Wave** | **203 µs** |
| **Whole request, the `net/http` baseline** | **221 µs** |

The framework's entire per-request bookkeeping is **2.7 µs** — request ID,
recovery, body limit, operational endpoints, session, security headers and CSRF
together, of which CSRF is most of it. One database query on loopback, against a
warm pool, is **37 µs**: fourteen times more. A real query crossing a network is
hundreds of microseconds to milliseconds, and against that the chain is not
visible.

The last two rows are the ones worth sitting with. The framework does strictly
more per request than the baseline — a session, a CSRF check, security headers,
a request ID, none of which the hand-written service has — and still spends
**less CPU** doing it, because the generated renderer beats `html/template` by
more than the middleware costs. Overhead that is real, fixed and small can still
be smaller than what it replaced.

The query row is the one that moved most recently. A PostgreSQL connection now
runs its request-time queries on the pgx-native pool rather than through
`database/sql`, which the framework's own benchmark measured at **34.9 µs/op and
15 allocations down to 23.0 µs/op and 8**. In this table the query row went from
49 µs to 37 µs, and end to end it was worth about ten points: the cookie
configuration went from 18% behind the baseline to 8%, and the server-side one
from level to 9% ahead.

Ten points from twelve microseconds off a 200 µs request is what the table
predicts, and that is the useful part. A per-operation number tells you what a
change is worth only once you know what share of the request that operation
holds. Quoted alone, "a third off the query" would have promised more.

## What the cookie session actually costs

The session is the part people expect to be expensive, because it encrypts. It
is worth pricing on its own.

The todo service's sealed session cookie carries a 215-byte payload. Measuring
`Keyring.seal` and `Keyring.open` directly — AES-GCM, ten runs each:

| Payload | Seal | Open |
| --- | --- | --- |
| 64 B | 357 ns | 120 ns |
| 256 B | 373 ns | 122 ns |
| 1 KB | 576 ns | 265 ns |
| 3 KB | 1.07 µs | 589 ns |

A request opens the incoming cookie and seals the outgoing one, so the crypto on
a typical session is **about 0.5 µs**, round trip. Even a 3 KB record near the
browser's cookie limit is 1.7 µs. A signed rather than sealed slot pays HMAC
instead: 393 ns to sign, 410 ns to verify.

Half a microsecond is one hundredth of a single loopback query, so the
encryption cannot be what separates the two rows at the top of this page. **The cookie's
price is bytes, not CPU.** A cookie-backed response carries 631 bytes of
`Set-Cookie` headers against a server-side session's 299 — the record itself
travels — and the browser sends all of it back on every subsequent request. That
is bandwidth, header parsing, and one more chance to overflow a packet, on every
request in both directions.

The practical reading: if a session is costing you throughput, move the record
to a server-side backend rather than trying to make the crypto cheaper. There is
no crypto worth optimising here.

Reproduce with:

```sh
go test ./session/ -bench Keyring -benchmem -count=10
```

## Three ways a simple benchmark flatters the baseline

**The largest factor is usually the configuration, not the code.** The
scaffolded `config.dev.toml` logs at debug and prints every statement the
database runs. On the todo service that is the difference between **3,425 and
8,700 requests per second** — the service spends more time describing its work
than doing it. This is the one number on this page that is not per-request CPU,
and it is still larger than everything else combined. Point a benchmark at a
production-shaped file before drawing any conclusion;
`examples/todo/popcornwave/config.bench.toml` is one.

**The two programs are not doing the same job.** Every framework request also
carries a session, a CSRF check, security headers and a request ID, and every
rendered form carries a token. The hand-written baseline has one middleware that
sets a header. Its HTML is 14 KB where the framework's is 27 KB, and the extra is
CSRF tokens across a hundred forms. Adding those to the baseline by hand is the
work the framework exists to remove, so a benchmark that omits them is measuring
the absence of features.

**Throughput and latency answer different questions.** Even in the cookie
configuration that is 8% behind on requests per second, HTML p95 is
**2.90–3.21 ms against the baseline's 3.15–3.39** — ahead — and with a
server-side session it is 22% ahead. Per-request CPU says the same and more:
**203 µs against 221 µs**, so the cookie configuration is behind on throughput
while spending less CPU per request. What it spends instead is network — 631
bytes of `Set-Cookie` on every response, returned on every request. If what your
users feel is latency, measure latency, and if it is throughput, look at bytes
before you look at code.

## What to tune in production

Five settings, in the order they repay attention.

### The session backend, first

It is the largest single lever, and the table at the top of this page is the
evidence: same code, 17 points apart. A cookie session puts the whole record on the
wire twice per request; a server-side one sends an identifier and keeps the
record where the bytes cost nothing.

```toml
[session]
enabled = true
backend = "rdb"   # or redis, dynamo, firestore
```

`dev-volatile` is what the measurement above used and it is **development only**
— the framework refuses it outside a development environment, because a restart
discards every session. For production the equivalent shapes are `rdb`, `redis`,
`dynamo`, and `firestore`. Each sends the same small identifier cookie, and each
adds a storage round trip the cookie backend does not have, so the win is not
free: you are trading bytes on every request for a lookup on every request.
Which way that goes depends on your storage latency, and it is worth measuring
rather than assuming. `redis` is the usual answer when the answer matters.

Stay on `cookie` when the deployment has nowhere to put a record, or when
sessions are small and you would rather not run another dependency.
[Sessions](/guides/backend/sessions/) covers the durability and revocation
differences, which usually decide this before performance does.

### The connection pool, second

`pw init` scaffolds ten connections, which is small for anything real:

```toml
[[middleware.rdb.connections]]
group = "default"
max_open_conns = 25
max_idle_conns = 25
```

Measured on the todo service, 10 → 25 was **+8.5%**, and 50 was slower than 25.
There is no universal number. The ceiling is what your database will accept
across every instance you run, and past it extra connections buy contention at
the server rather than throughput at the client. Start at 25, measure, and
multiply by your replica count before asking whether PostgreSQL can take it.

Set `max_idle_conns` equal to `max_open_conns` unless you have a reason not to.
A lower idle count closes and reopens connections under fluctuating load, and a
TLS handshake per reconnect costs far more than an idle socket.

### Logging, third

The production defaults are already right — `minimum_level` resolves to `info`
and query logging to off outside dev — so this is a matter of not overriding
them. If you turned statement logging on to debug something, turn it off again.
See [Query Diagnostics](/productivity/query-diagnostics/).

### Compression, fourth

Off by default, and usually correctly so, because something in front of the
application already compresses.
[Response Compression](/guides/frontend/compression/) covers when that is not
true.

### CSRF scope, last

At 2.4 µs it is the most expensive link in the chain, and the chain is 4.5 µs,
so this is a rounding error unless you are certain it is not. A token is issued
only for a request that looks like an HTML document — `Accept: text/html` or
`Sec-Fetch-Dest: document` — so an API read already skips the work without any
configuration. A load generator that omits those headers is not measuring the
page path, and will see a form render fail for want of a token. `include` defaults
to every path; an API authenticating with a bearer token and holding no session
does not need the check:

```toml
[security.csrf]
enabled = true
include = ["/**"]
exclude = ["/api/**"]
```

Only exclude paths that genuinely cannot be driven by a browser carrying a
cookie. Getting this wrong reintroduces the vulnerability the check exists for,
in exchange for 2.4 µs.

## When none of this is enough

If profiling shows the HTTP stack itself is your limit, `fasthttp` is designed
for that and Popcorn Wave does not compete with it — see
[Why Popcorn Wave](/start/why-popcorn-wave/#when-the-http-stack-is-the-measured-bottleneck).
Reaching that boundary means giving up the `net/http` ecosystem, so confirm it
with a measurement rather than an assumption.

## Measuring your own application

Add a profiler on its own listener so the mux under test is unchanged. Both todo
services do this in `pprof.go`, gated on an environment variable.

Pick the profile that answers your question, because the wrong one misleads
confidently:

| Question | Instrument |
| --- | --- |
| Which functions burn CPU? | CPU profile |
| Am I waiting on a lock? | mutex profile, via `SetMutexProfileFraction` |
| Where do goroutines park? | block profile, via `SetBlockProfileRate` |
| Is this layer the cause? | swap that layer and re-measure |

The last row is not a profile, and it is the one that settles arguments. A CPU
profile of the todo service shows `database/sql.withLock` at 15% of samples,
which reads like a serialization problem. Replacing the baseline's driver with
that same `database/sql` layer cost it 3%, so the lock was not the limit. Two
things make that reading wrong: a CPU profile cannot show contention at all,
because a goroutine waiting on a mutex is parked and never sampled, and a
cumulative figure includes everything the function called — here, the driver
running the query. Both errors point the same wrong way.
