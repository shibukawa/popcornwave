---
title: Architecture
description: The request model Popcorn Wave is built around, what is generated ahead of time, and what a minimal build excludes.
sidebar:
  order: 3
---

Popcorn Wave starts with a familiar constraint: a server-rendered application
should still look like a `net/http` application. Its optional server-driven UI
layer extends that foundation, but nothing described here depends on it. If an
application does not import the extra runtime, it does not pay for it.

## The request model

An HTTP handler binds one request, runs the application logic, and writes one
complete response. Navigation and forms keep their standard browser semantics.

| Concern | How it works |
| --- | --- |
| Unit of work | one `http.Handler` |
| Links | full document requests |
| Forms | ordinary submissions and redirects |
| Mutation | handler or application service |
| Browser default after a mutation | Post/Redirect/Get |
| Transaction boundary | explicit, via `pw.Transaction` |
| Client-side enhancement | optional |

The model is deliberately unsurprising. A handler that a Go developer already
knows how to read remains that same handler after the framework is added.

## Generation instead of reflection

That familiar request model does not rule out stronger checks. `pw generate`
reads your sources ahead of time and writes Go beside them:

| Source | Generated |
| --- | --- |
| `*.pw.html` | typed component functions and parameter structs |
| `*.pw.sql` | typed, context-taking query functions |
| `pw.Parse[T]` call sites | request binding for `T` |
| `pw.WriteAPI[T]` / `pw.NewStream[T]` call sites | response encoding for `T` |
| `pw.RegisterConfig[T]` call sites | configuration binding for `T` |
| all of the above | an OpenAPI 3.1 fragment |

This changes when mistakes appear. A template that inserts a `string` into a URL
attribute, a query whose SELECT columns do not match its result type, or a
handler that omits a new template parameter fails at build time. The same
generation step removes runtime reflection from the hot path, making the
framework a practical **TinyGo** target.

Escaping is contextual and automatic, with a deliberately narrow raw-output
escape hatch. Status and headers are chosen before the body is written, and
HTML is rendered into a buffer and validated before anything is committed.

## Layering

The generated layers come from [`tinybind-go`](https://github.com/shibukawa/tinybind-go),
which was built for this framework:

| Layer | Responsibility |
| --- | --- |
| `httpbind` | request binding, validation, JSON and streaming responses, OpenAPI |
| `htmlbind` | typed HTML components and render chains |
| `sqlbind` | typed SQL statements and result scanning |
| `configbind` | configuration binding, scaffolds, subcommands |

Popcorn Wave wraps these behind the `pw` package, so application code imports
one stable API rather than four.

## Standard library compatibility

`pw.NewServeMux` returns `*http.ServeMux` on ordinary Go builds — it is a type
alias, not a wrapper. Patterns, wildcards, and precedence are exactly the
standard library's. A separate implementation with the same semantics is
compiled in only for TinyGo, which lacks the standard mux.

The same holds throughout: handlers are `http.HandlerFunc`, middleware is
`func(http.Handler) http.Handler`, and `pw.Middlewares` hands back a plain
`http.Handler` if you want to own the server yourself.

## Caching

Three layers are available and independent:

- HTTP validators and the response cache
- a safe generated read-query cache
- an application cache

## What a minimal build excludes

A build that uses only what is described in these guides is meant to stay a
small, practical TinyGo target:

- it works without the Popcorn Wave browser runtime;
- it uses standard requests and complete responses;
- it interoperates with `net/http` handlers and middleware;
- it supports typed binding, configuration, errors, templates, and OpenAPI;
- it optionally supports Tailwind CSS, still without a browser runtime;
- it excludes component graph, patch protocol, and hydration dependencies.

The final exclusion defines the boundary. The server-driven UI layer may extend
this foundation, but it never becomes a prerequisite for it.

## Next steps

- [Handlers](/guides/frontend/handlers/)
- [Responses](/guides/frontend/responses/)
