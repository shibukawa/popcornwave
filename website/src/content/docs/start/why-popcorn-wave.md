---
title: Why Popcorn Wave
description: Raise productivity and build fast services without giving up the conventions Go developers already share.
sidebar:
  order: 1
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
configuration declarations into typed Go. It also derives OpenAPI from the same
sources. A changed query result or a missing template argument therefore fails
during generation or compilation, before a request reaches production.

The command-line tools remove a different kind of repetition. `pw init` creates
a runnable project, `pw dev` watches its inputs, and the generated project
already has startup validation, operational endpoints, graceful shutdown,
structured errors, and security headers. These features stay visible as Go and
configuration rather than disappearing behind a separate application model.

## The productive path is also a fast path

Code generation moves work out of the request path. Templates call typed
functions, generated binders decode into known fields, and generated queries
scan known columns; the runtime does not rediscover those shapes with
reflection on every request. Optional browser features are separate imports, so
a service that does not use them does not carry their runtime.

Popcorn Wave is built on `net/http`, whose server, tooling, middleware
conventions, and profiling support are already part of the Go ecosystem. That
is the intended default: enough performance for a broad range of services,
without exchanging interoperability for a benchmark result the application may
never need.

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
