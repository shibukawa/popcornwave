---
title: Why Popcorn Wave
description: Make secure, fast, operable services the natural result of a productive Go workflow, without giving up the conventions Go developers already share.
sidebar:
  order: 0
---

Popcorn Wave exists to raise productivity and build fast services while keeping
the conventions Go developers already share. An application still looks like
Go: `net/http` handlers, `context.Context`, ordinary middleware, and
`database/sql` remain the boundaries a team reads, tests, and operates.

That familiarity matters after the first week. A handler written for Popcorn
Wave can be wrapped by standard middleware, mounted beside another
`http.Handler`, or tested with `httptest`. Adopting the framework does not also
mean teaching the codebase a private request model.

## Productivity comes from removing repetition

Keeping ordinary Go does not mean writing every mechanical layer by hand.
`pw generate` turns templates, SQL, request binding, response types, and
configuration declarations into typed Go. A changed query result or a missing
template argument therefore fails during generation or compilation, before a
request reaches production.

The command-line tools remove a different kind of repetition. `pw init` creates
a runnable project, `pw dev` watches its inputs, and the generated project
already has startup validation, operational endpoints, graceful shutdown,
and structured errors. These features stay visible as Go and configuration
rather than disappearing behind a separate application model.

## A simple implementation should not be a bare one

A short handler is attractive. A service that is short only because
authentication, response policy, compression, observability, and configuration
checks were left for later is not. Popcorn Wave keeps those concerns on the
supported path, so a small application can gain them without growing a second
framework around itself:

- [Federated login and passkeys](/guides/backend/authentication/) provide both
  external identity and phishing-resistant repeat login.
- [Security response headers](/guides/frontend/security-headers/) establish
  browser policy by default, while one switch enables negotiated
  [zstd compression](/guides/frontend/compression/) when the application rather
  than a proxy owns compression.
- Static analysis turns the handlers, bindings, response calls, and comments
  already in the code into [OpenAPI 3.1 documentation](/productivity/api-documentation/).
- [OpenTelemetry integration](/guides/cross-layer/tracing/) exports structured
  logs and traces, while framework spans make requests, renders, and database
  statements visible without application-specific timers.
- Typed [configuration](/guides/architecture/configuration/) accepts environment
  variables, and [`pw doctor`](/pw/project/doctor/) resolves each named
  environment and reports missing, conflicting, or unsafe settings before
  deployment.

Some of these mechanisms remain deliberate choices: compression is off when a
proxy should own it, and federated login still needs a provider. What changes is
the starting point. The straightforward implementation becomes naturally more
secure, faster over the wire, observable, and easier to develop and operate.

The JavaScript ecosystem has been relentless in its pursuit of productivity;
the Go ecosystem has favored stability. Popcorn Wave is not trying to split the
difference. Its aim is to keep Go's stable interfaces and predictable
operations, close the gap, and eventually move beyond the productivity bar the
JavaScript ecosystem has set. Generated checks, secure defaults, and integrated
tooling make that the more productive way to build.

## The productive path is also a fast path

Code generation moves work out of the request path. Templates call typed
functions, generated binders decode into known fields, and generated queries
scan known columns; the runtime does not rediscover those shapes with
reflection on every request. Optional browser features are separate imports, so
a service that does not use them does not carry their runtime.

Rendering is where that shows up most plainly. The same inventory table written
twice — once as an `html/template` parsed at startup, once as a typed component
— rendered to the same bytes. Medians of fifteen runs on Go 1.26.5, Apple M3:

| Rows | `html/template` | Generated |
| --- | --- | --- |
| 1 | 2.55 µs · 1032 B · 41 allocs | 393 ns · 560 B · 5 allocs |
| 50 | 78.5 µs · 26.1 KiB · 1282 allocs | 6.82 µs · 611 B · 21 allocs |
| 500 | 780 µs · 259 KiB · 12 956 allocs | 70.4 µs · 2.14 KiB · 472 allocs |

The memory column is the one that compounds. A fifty-row page costs 611 bytes
instead of 26 KiB, and what is never allocated is never collected later — a cost
the collector otherwise charges to whichever request happens to be running. The
generated JSON codec has the same shape at a smaller scale, decoding a six-field
request in 184 ns against 998 and encoding it in 74 against 160, though inside a
whole HTTP request that difference largely disappears into the request itself.

Popcorn Wave is built on `net/http`, whose server, tooling, middleware
conventions, and profiling support are already part of the Go ecosystem. That
is the intended default: enough performance for a broad range of services,
without exchanging interoperability for a benchmark result the application may
never need.

## TinyGo is a first-class target

Removing reflection has a second consequence, and it is the one the project is
betting on. If WebAssembly becomes an ordinary place to deploy server code,
TinyGo is the compiler that gets you there — and `html/template` does not run
there at all. It compiles, then panics during package initialization on
reflection TinyGo does not implement, before a single byte is rendered. A
generated component has nothing to reflect over and simply runs.

Most of what a Go server reaches for has the same problem: `crypto/tls`, the Go
1.22 `ServeMux`, `aws-sdk-go-v2`, the Google client libraries. Rather than treat
TinyGo as a degraded mode, the dependencies were rebuilt.
[`tinygodriver`](https://github.com/shibukawa/tinygodriver) supplies a host
network driver, HTTPS over the operating system's TLS stack, `database/sql`
drivers for SQLite, PostgreSQL and MySQL, and clients for S3, DynamoDB and
Datastore. That is more infrastructure than one person would sensibly take on,
and it exists because AI assistance made it tractable.

## When the HTTP stack is the measured bottleneck

Some services need more performance than `net/http` provides. If profiling
shows that the HTTP stack itself is the limiting factor, `fasthttp` is a
reasonable tool designed for that requirement. It uses a different request API
and ecosystem boundary, so Popcorn Wave does not put it underneath a
`net/http`-shaped compatibility layer.

This is not a judgment against `fasthttp`; it is a boundary. Popcorn Wave
optimizes the path where standard Go compatibility, development speed, and
strong generated checks must coexist. A service whose measurements justify a
different boundary should choose the boundary it needs.
