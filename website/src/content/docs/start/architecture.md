---
title: Architecture
description: The request model Popcorn Wave is built around, what is generated ahead of time, and what a minimal build excludes.
sidebar:
  order: 3
---

Popcorn Wave is a server-rendered web framework built directly on `net/http`.
An optional server-driven UI layer sits on the same foundation and extends it;
everything described here works without that layer, and an application never
pays for a runtime it does not import.

## The request model

An HTTP handler binds one request, runs application logic, and writes one
complete response, using standard navigation and form semantics.

| Concern | How it works |
| --- | --- |
| Unit of work | one `http.Handler` |
| Links | full document requests |
| Forms | ordinary submissions and redirects |
| Mutation | handler or application service |
| Browser default after a mutation | Post/Redirect/Get |
| Transaction boundary | explicit, via `pw.Transaction` |
| Client-side enhancement | optional |

Nothing here is novel — that is the point. A handler a Go developer can already
read stays a handler you can already read.

## Generation instead of reflection

The framework leans on ahead-of-time generation. `pw generate` reads your
sources and writes Go beside them:

| Source | Generated |
| --- | --- |
| `*.pw.html` | typed component functions and parameter structs |
| `*.pw.sql` | typed, context-taking query functions |
| `pw.Parse[T]` call sites | request binding for `T` |
| `pw.WriteAPI[T]` / `pw.NewStream[T]` call sites | response encoding for `T` |
| `pw.RegisterConfig[T]` call sites | configuration binding for `T` |
| all of the above | an OpenAPI 3.1 fragment |

Two consequences follow. First, mistakes surface at build time: a template that
inserts a `string` into a URL attribute, a query whose SELECT columns do not
match its result type, a handler that forgot a new template parameter. Second,
there is no runtime reflection in the hot path — which is what makes the
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

That last line is the load-bearing one. The server-driven UI layer extends this
foundation; it is never a prerequisite for it.

## Next steps

- [Handlers](/guides/handlers/)
- [Responses](/guides/responses/)
