# Feature specification — request rate limiting

Hand this to the agent as the specification for a feature that has just landed,
and ask for the documentation. Nothing here is implemented; it is a fixture.

## What shipped

`middlewares/ratelimit.go`, plus a `[middleware.ratelimit]` configuration block.

The middleware counts requests per key and rejects the ones over the limit with
`429 Too Many Requests` and a `Retry-After` header. It sits in the middleware
chain like `middlewares/bodylimit.go` does, so it applies to every route on the
mux and there is no per-route form.

## Configuration

```toml
[middleware.ratelimit]
enabled = false
rate = 100
window = "1m"
burst = 20
key = "ip"
store = "memory"
skip_paths = ["/healthz", "/readyz"]
trust_proxy = false
```

`rate` and `window` set the sustained allowance. `burst` is how far above it a
client may go momentarily; the implementation is a token bucket and there is no
choice of algorithm.

`key` is `ip` or `session`. Keying on the session requires a session middleware
earlier in the chain; with no session, a request falls back to its IP.

`store` is `memory` or `redis`. The memory store keeps counters in the process,
so two instances behind a load balancer enforce two separate limits — the
effective limit is the configured one multiplied by the instance count. The
redis store shares counters across instances and reads its connection from the
existing `[middleware.redis]` block. Devbox already pins Valkey, so a project
scaffolded by `pw init` has a server running.

`trust_proxy = false` means the client IP is the socket peer. Setting it true
reads `X-Forwarded-For`, which is only safe when something you control sets that
header — otherwise a client can forge it and get an unlimited allowance.

`skip_paths` bypasses the limiter entirely for the listed prefixes. The
operational endpoints are in the default because a limiter that rejects
`/readyz` takes the instance out of the load balancer under exactly the load it
was meant to survive.

## Behaviour

Every response carries `RateLimit-Limit`, `RateLimit-Remaining`, and
`RateLimit-Reset`. A rejected request also carries `Retry-After` in seconds, and
its body is a `problem+json` document written through `pw.WriteProblem`, so it
matches every other framework-level rejection.

The limiter runs before the handler and after the access log, so a rejected
request is logged.

## Known limits

The memory store's counters are per-process and are lost on restart, which
briefly doubles the allowance during a rolling deploy.

Redis being unreachable fails **open** — requests pass unlimited and the failure
is logged at warn level. Refusing traffic because the limiter's own dependency is
down was judged worse than not limiting.

There is no per-route or per-user override. A route needing a different limit
has to implement it itself.
