---
title: Rate Limiting
description: Two bounded request buckets and a process ceiling behind your edge proxy, and the reasoning for keeping all three that small.
sidebar:
  order: 8
---

```toml
[ratelimit]
enabled = true
```

With that, each caller gets a budget of requests per minute — 600 for an
authenticated user, 300 for an anonymous address — and a request over budget is
refused with `429`, `Retry-After`, and the `X-RateLimit-*` headers. A browser
sees your application's 429 page; an API client gets a problem document. The
counters live in process, which is the right default for one replica and the
wrong one for more; [the Redis backend](#counting-across-replicas) fixes that.

It is off by default because the proxy or API gateway in front of a normal
deployment already limits by address, and a second limiter that nobody sized is
noise. Turn it on for the budget the edge cannot compute: a limit per
*authenticated user*, which requires resolving the session — something only the
application can do.

## Two buckets, not a rule engine

Every counted request lands in exactly one bucket. If the request carries a
verified identity, it spends from that subject's budget (`per_subject`);
otherwise it spends from its client address's (`per_address`). There are no
per-route rules and no pattern grammar — a caller has one budget across the
whole application, and a per-operation quota belongs in the API gateway that
already has the vocabulary for it.

The two numbers are sized against different populations. An authenticated
caller is accountable — you know who they are and can revoke them — so
`per_subject` can be generous, and setting it to `0` disables that bucket
entirely. The address bucket has no off position: it is the only thing an
unauthenticated flood meets, and it is also what a corporate NAT shares among
many real people. Startup refuses `per_address = 0` rather than treating it as
"unlimited", because for this bucket an unlimited value is an absent control.

```toml
[ratelimit]
enabled = true
window = "1m"
per_subject = 600
per_address = 300
```

The algorithm is a fixed window: the counts reset when the window rolls over,
and `X-RateLimit-Reset` tells the client when. A determined caller can spend
two windows' budget across the boundary — that burst is a doubled allowance,
not a flood, and volumetric defence stays the edge's job.

Behind a proxy, set
[`server.trusted_proxies`](/reference/configuration/#server) or every
anonymous caller resolves to the proxy's address and shares one bucket, which
turns the limit into an outage.

## The process ceiling

`process` is a third count with no key at all: total arrivals per window, for
the whole process.

```toml
[ratelimit]
enabled = true
process = 30000
```

It exists because a distributed flood keeps every source under any per-address
value by construction — many addresses, each politely under budget — so no
keyed bucket can see it. The unkeyed ceiling is the only layer here that can.

It defaults to `0` (off) rather than to a guess, because it is the most
dangerous number in this section: size it against what your deployment can
actually serve, not against an attack, since a ceiling below real capacity
refuses legitimate traffic globally. Startup refuses a `per_address` or
`per_subject` larger than a positive `process` — a caller allowed more than
the total describes a limit that can never bind.

## Counting across replicas

The default `memory` backend counts inside one process, so N replicas enforce
N times the configured limit. That is stated here rather than discovered in
production. For more than one replica, count in Redis or Valkey:

```toml
[ratelimit]
enabled = true
backend = "redis"
redis.dsn = "${RATELIMIT_REDIS_DSN}"
```

```go
import _ "github.com/shibukawa/popcornwave/ratelimitstore/redis"
```

The blank import links the backend; registration opens no connection, and the
client dials when `backend = "redis"` selects it. An unreachable server
refuses startup — shipping a limiter that silently fails open on every request
would be worse than not starting. Redis keys under a `memory` backend are
refused rather than ignored, so a half-switched configuration is caught at
startup too.

## What is never counted

The framework's own operational endpoints — health, readiness, the OpenAPI
document and its viewer — and the [public asset
mount](/guides/frontend/static-assets/) are exempt, and the carve-out is not
configurable. A readiness probe arrives from the proxy on the same address as
every anonymous caller and would exhaust that bucket by itself, and one page
view fetches many assets; counting either turns the limit into an outage on
the first deploy.

## When the store fails

An unreachable counter store admits the request and logs at error level that a
request was admitted without being counted. This is deliberate: the edge still
has its own limits, so failing open degrades to edge-only protection, while
failing closed would convert a Redis blip into an outage of every limited
route at once — including login. There is no switch for this. What deserves
your attention is the log line, because a limiter that is silently not
limiting is the worst of the three states.

## Every key

The full key list, defaults, and the environment-variable forms are in the
[configuration reference](/reference/configuration/#ratelimit).
