---
title: Architecture
description: The request model Popcorn Wave is built around, what is generated ahead of time, and what a minimal build excludes.
sidebar:
  order: 2
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

## What runs, and where it came from

Every part that moves during one request is one of four things: code you wrote,
code `pw generate` wrote from something you wrote, the framework's own code, or
the standard library's. The picture is a whole request, coloured by that
question.

![A request rising through http.Server, the framework's middlewares, and http.ServeMux into a handler that calls a generated binder, query function, and component function before the runtime writes the response](../../../assets/diagrams/request-parts.svg)

Read it from the bottom. `http.Server` accepts the connection, the framework's
middlewares wrap the whole mux rather than each route, and `http.ServeMux` picks
the handler. Nothing on that path is a Popcorn Wave type — `pw.Run` hands the
server a plain `http.Handler` and steps out of the way.

Your handler is where generated code starts being called. Three of the four
things it calls were written by `pw generate`, and each one has a source
directly above it that you own.

**The request binder** comes from the struct you declared. Its field tags say
where each field is read from and what has to be true of it, and the binder
generated from them is registered under that struct's type, which is how
`pw.Parse[Input]` finds it. The field mapping is generated code rather than
reflection over your struct.

**The query function** comes from a `.pw.sql` file. Each named statement in it
becomes one exported Go function whose arguments are the statement's parameters
and whose rows are scanned into the record type declared beside it, so a column
renamed in a migration stops the build rather than a request.

**The component function** comes from a `.pw.html` template. It takes a
generated parameter struct and returns a fragment rather than a string, so a
parameter the handler forgot is a compile error, and the escaping for every
position was chosen while the template was read — contextually, with a
deliberately narrow raw-output escape hatch.

The fourth is not generated. `pw.WriteHTML` is the framework's own runtime: it
renders the fragment into a buffer and validates it, and only then commits
status, headers, and body. Status and headers are therefore always decided
before the first byte of the body goes out.

Generation pays for itself twice over. Mistakes move to build time: a template
that inserts a `string` into a URL attribute, a query whose SELECT columns do
not match its result type, and a handler that omits a new template parameter all
stop the compiler. And the request path is left with no reflection in it, which
is what makes **TinyGo** a practical target.

The generated code is not ours either. It comes from
[`tinybind-go`](https://github.com/shibukawa/tinybind-go), which was built for
this framework, and the three generated boxes are three of its four layers:

| The part in the picture | The layer behind it | What else that layer covers |
| --- | --- | --- |
| request binder | `httpbind` | validation, JSON and streaming responses, OpenAPI |
| query function | `sqlbind` | statement building, result scanning |
| component function | `htmlbind` | render chains, contextual escaping |
| — | `configbind` | configuration binding, scaffolds, subcommands |

`configbind` has no box because its work is finished before the first request
arrives: configuration is bound once at startup. Popcorn Wave wraps all four
behind `pw`, so application code imports one stable API rather than four —
`pw.HTMLFragment` *is* `htmlbind.Fragment`, and the generated files are the only
place those four names appear.

## What `pw generate` writes

The picture shows the three parts a handler calls directly. The full list is
shorter than the machinery behind it:

| Source | Generated |
| --- | --- |
| `*.pw.html` | typed component functions and parameter structs |
| `*.pw.sql` | typed, context-taking query functions |
| `pw.Parse[T]` call sites | request binding for `T` |
| `pw.WriteAPI[T]` / `pw.NewStream[T]` call sites | response encoding for `T` |
| `pw.RegisterConfig[T]` call sites | configuration binding for `T` |
| all of the above | an OpenAPI 3.1 fragment |

All of it lands beside the source it came from, in files ending `_pw_gen.go`.
Those are build output: `pw generate` overwrites them, `pw dev` reruns it
whenever a source changes, and nobody edits them by hand.

## Standard library compatibility

`pw.NewServeMux` returns `*http.ServeMux` on ordinary Go builds — it is a type
alias, not a wrapper. Patterns, wildcards, and precedence are exactly the
standard library's. A separate implementation with the same semantics is
compiled in only for TinyGo, whose own `ServeMux` does not yet support the
Go 1.22 method and path-parameter patterns as of TinyGo 0.41.

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
