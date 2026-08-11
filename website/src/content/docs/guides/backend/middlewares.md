---
title: Middlewares
description: What the framework wraps around every request, in what order, and where your own middleware fits into the stack.
sidebar:
  order: 4
---

Every request passes through a stack you mostly never see. That invisibility is
the point — you did not install the panic handler, and it catches your panics
anyway — right up until a response is missing a header, a log line is missing
its request ID, or a health probe answers `503` and you need to know what sits
between the socket and your handler. This page is that map: what is installed,
in what order, which switch moves each piece, and how your own middleware
takes a numbered position in the same stack.

## The stack, outside in

Every frame sits on one number line, ascending from the outside in, and the
framework's own frames occupy the multiples of ten — BASIC line numbers, and
for the same reason: the gaps are yours. Each number below has an exported
constant (`pw.SlotRequestID`, `pw.SlotAccessLog`, …), so a position is written
relative to a name rather than as a bare integer.

| Slot | Frame | What it does | Switch |
| --- | --- | --- | --- |
| — | request tracking | counts in-flight requests for graceful shutdown | always on |
| 10 | OpenTelemetry | opens the request root span | only when tracing exports |
| 20 | resource injection | puts the logger, database, and config into the context | always on |
| 30 | request ID | validates or mints the ID every log line carries | `middleware.request_id` |
| 40 | access log | one structured line per request, with timing | `middleware.access_log` |
| 50 | recover | converts a panic into a negotiated error response | `middleware.recovery` |
| 60 | security headers | CSP, HSTS, and friends, before anything writes | `security.headers.enabled` |
| 70 | request timeout | bounds the whole request | `middleware.request_timeout` |
| 80 | body limit | caps downstream reads of the request body | `server.max_request_body` |
| 90 | public assets | serves the static tree before dynamic work | `server.public.enabled` |
| 100 | probes | health and readiness, above everything that authenticates | `server.health`, `server.readiness` |
| 110–150 | extensions | storage, session, authentication, CSRF, guard | per extension |
| 160 | API documentation | the OpenAPI document and its UI, behind the guard | `server.openapi`, `server.apidoc` |
| — | your handler | the mux you passed to `pw.Run` | — |

Request tracking stays off the line, outermost, because its shutdown
accounting must observe every numbered step; the handler is the innermost end
by definition. Everything between them is ordered by its number alone.

The order is not alphabetical and not historical; each position is an argument.
The request ID sits outside the access log so the log line can carry it. The
access log sits outside recover so a request that panics still appears, with
its timing and a `500`. The probes sit above the extension chain so a liveness
check succeeds while the session store is down — a dependency outage should
fail readiness, not turn into a restart loop. And the OpenAPI document sits
*below* the guard, because a map of your whole API surface deserves the same
protection as the routes it describes.

Compression is configured beside these (`middleware.compression`) but is not a
frame in the chain: it is applied where the response is written. The
[compression guide](/guides/backend/compression/) covers when to turn it on.

## The extension slots

The stretch from 110 to 150 is not hard-coded either. Imported capabilities
register themselves into it, and the chain assembles them by the same numbers:

| Slot | Constant | Installed by |
| --- | --- | --- |
| 110 | `pw.SlotStorage` | session backends that open storage clients |
| 120 | `pw.SlotSession` | the framework's session resolution |
| 130 | `pw.SlotAuthentication` | `plugin/auth` |
| 140 | `pw.SlotCSRF` | the CSRF check, where `security.csrf` enables it |
| 150 | `pw.SlotGuard` | the authentication guard |

The numbering is deliberate: a guard at 150 always observes the session
resolved at 120 and the authentication finalized at 130. A package that
provides a capability — a reusable component package, or your own internal
one — registers from its `init`:

```go
package audit

import (
	"context"
	"net/http"

	"github.com/shibukawa/popcornwave/pw"
)

func init() {
	pw.RegisterExtension(pw.Extension{
		Name: "audit",
		Slot: pw.SlotGuard + 1, // after authentication, after the guard
		Setup: func(ctx context.Context) (pw.Middleware, error) {
			return func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// The session and authentication are resolved above us.
					next.ServeHTTP(w, r)
				})
			}, nil
		},
	})
}
```

`Setup` runs once at startup, after configuration parsing and database startup,
and receives the same resources handlers see — so a misconfigured extension
fails the boot, not the first request. Returning a nil middleware installs
nothing, which is how a disabled capability opts out.

## Writing your own middleware

`pw.RegisterMiddleware` takes a slot, a name, and a plain
`func(http.Handler) http.Handler`, and the chain that `pw.Run` and
`pw.Middlewares` build includes it at that position. Call it from `main`,
after every package `init` has run and before the chain is built — the same
timing `pw.RegisterSessionStore` asks for, and for the same reason: the chain
is composed once, and a middleware registered later joins nothing.

The most useful thing a small middleware can do is derive a per-request fact
once and let everything below read it. The
[`session.RequestScope`](/guides/backend/sessions/) placement exists for
exactly this, and the request clock is the standing example. Handlers that
call `time.Now()` at each write scatter timestamps across the request: three
rows updated by one form submission carry three different `updated_at` values,
drifting by however long the handler took between writes. Capture the moment
once instead — here is the whole program, from the type to the registration:

```go
// cmd/myapp/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/shibukawa/popcornwave/pw"
	"github.com/shibukawa/popcornwave/session"

	"myapp/handlers"
)

type RequestTime struct {
	At time.Time `json:"at"`
}

func withRequestTime(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handle, ok := session.Value[RequestTime](r.Context()); ok {
			handle.Set(RequestTime{At: time.Now()})
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// The middleware writes into session state, so it sits below the
	// session resolution at 120.
	pw.RegisterSessionStore[RequestTime]("request_time", session.RequestScope)
	pw.RegisterMiddleware(pw.SlotSession+5, "request_time", withRequestTime)

	if err := pw.Run(context.Background(), handlers.Handlers()); err != nil {
		log.Fatal(err)
	}
}
```

Every write in the request then reads `session.Load[RequestTime]` for its
`updated_at`, and one submission stamps one moment. The same shape serves any
fact that is true for exactly one request — the scope set a bearer token
resolves to, a feature-flag snapshot taken at the top so the request cannot see
a flag flip halfway through.

The one decision the example leaves open is the number, and you pick it by
what the middleware needs to observe. Below 20 there are no resources in the
context; below 50 a panic is answered by recover; below 120 the session is
resolved; after 150 only requests the guard admitted arrive. The request clock
sits at `pw.SlotSession+5` because it writes into session state, which does
not exist above 120. A middleware that only reads headers can sit much
higher — at `pw.SlotAccessLog-5`, say, where the request ID at 30 is already
minted and the access log at 40 will time it. Two middlewares at one number
run in registration order, so a shared slot is fine when the pair is
order-independent.

Two positions refuse registration: 100 and 160, the probes and the API
documentation. They are handlers rather than middleware — nothing can share
their exact position — so the panic names the constant to move relative to.

One seam remains outside the line: wrapping what `pw.Middlewares` returns, for
the rare middleware that must observe requests the framework would otherwise
answer, probes included:

```go
handler, err := pw.Middlewares(mux)
if err != nil {
	log.Fatal(err)
}
err = http.ListenAndServe(":8080", myOutermost(handler))
```

That position trades away everything the stack provides — no request ID, no
recover, no resources in the context — so reach for it only when observing the
raw request is the point. For everything else, a number on the line says what
you mean and the chain enforces it.
