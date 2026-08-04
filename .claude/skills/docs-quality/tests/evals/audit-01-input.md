---
title: Rate Limiting
description: Rate limiting.
sidebar:
  order: 4
  badge: advanced
---

Popcorn Wave provides rate limiting. Rate limiting is a middleware that limits
requests. This page explains rate limiting.

## Options

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `enabled` | bool | `false` | Enables the middleware |
| `algorithm` | string | `"token_bucket"` | The algorithm |
| `rate` | int | `100` | Requests per window |
| `window` | duration | `"1m"` | The window |
| `burst` | int | `20` | Burst allowance |
| `key` | string | `"ip"` | What to key on |
| `store` | string | `"memory"` | Where counters live |
| `redis_addr` | string | `""` | Redis address |
| `redis_db` | int | `0` | Redis database |
| `header_mode` | string | `"standard"` | Which headers to send |
| `status` | int | `429` | Status code on rejection |
| `skip_paths` | []string | `[]` | Paths that bypass the limiter |
| `trust_proxy` | bool | `false` | Read the client IP from headers |

There are three algorithms available:

- `token_bucket` — a token bucket
- `sliding_window` — a sliding window
- `fixed_window` — a fixed window

There are three keying strategies:

- `ip` — keys on the client IP
- `session` — keys on the session
- `header` — keys on a header

All of them work. Choose the one that fits your use case.

## Usage

```go
mux.Use(ratelimit.New(cfg))
```

## Stores

| Store | Shared across instances | Survives restart |
| --- | --- | --- |
| `memory` | no | no |
| `redis` | yes | yes |

The memory store is faster. The redis store is shared.

## Notes

- Works with both the classic router and the modern router.
- The `pw.ServeMux` wrapper passes requests through the middleware chain.
- See [configuration](/guides/configuration/) for the full key list.
- Generated code is written to `ratelimit_pw_gen.go`; edit it if you need to
  change the behaviour.

```html
<div class="rounded-lg bg-red-50 p-4">Too many requests</div>
```
