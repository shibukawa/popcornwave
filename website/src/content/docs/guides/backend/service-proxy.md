---
title: Proxying to a Backend Service
description: Mount a reverse proxy handler on your own mux so an existing API server answers at the same origin, behind the framework's session and authentication.
sidebar:
  order: 5
---

An API server already exists. It is written in another language, or it is a
service someone else maintains, and the browser needs to reach it from the same
pages this application renders. Serving it from a second hostname works, and
costs a CORS preflight on every unsafe request plus a second answer to the
question of who is logged in. That path is available — [Cross-Origin
Requests](/guides/backend/cors/) configures it — and the second answer is the
part it leaves you maintaining. Putting both behind nginx solves the origin and
not the login: the proxy hands the request straight to the API, which now needs
its own idea of a session.

Mount the proxy inside the application instead and the request arrives here
first. It passes the whole middleware stack — session resolution,
authentication, the CSRF check, the body limit, the request timeout — and only
then goes upstream. The service behind you can keep having no authentication at
all, because nothing reaches it that has not already been through yours.

## The handler

`net/http/httputil` does not build under TinyGo, so the proxy comes from
`tinygodriver/httprevproxy`, whose public API mirrors the reverse-proxy portion
of the standard package. Every Popcorn Wave project already requires
`tinygodriver` — `pw.ServeMux` is one of its types — so there is no dependency
to add. Import it under the `httputil` alias and the call sites read exactly as they
would against the standard package.

```go
package handlers

import (
	"net/url"

	httputil "github.com/shibukawa/tinygodriver/httprevproxy"
)

// The API server this application fronts. It listens on loopback and has no
// authentication of its own.
var apiTarget = &url.URL{Scheme: "http", Host: "127.0.0.1:9000"}

func init() {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(apiTarget)
			r.SetXForwarded()
		},
	}
	mux.Handle("/api/", proxy)
}
```

That file goes in the handlers package beside your other handlers, and `mux` is
the same `pw.NewServeMux()` they register on. `GET /api/users` now reaches the
API server, and so does everything else under the prefix — a trailing slash is a
subtree pattern on `pw.ServeMux` exactly as it is on `net/http.ServeMux`.

## What the request has already been through

A handler on the mux is the innermost end of the
[middleware stack](/guides/backend/middlewares/), so a request reaching the proxy
has been through every frame above it. That is the whole reason to proxy here
rather than in front:

| | What it means for the upstream |
| --- | --- |
| session and authentication | an unauthenticated caller never reaches the service |
| path protection and the guard | route-level authorization applies to proxied paths too |
| `server.max_request_body` | an oversized upload is refused before a byte is forwarded |
| `middleware.request_timeout` | bounds the upstream round trip, not just your own handlers |
| request ID and access log | one log line per proxied request, correlated with the rest |

The upstream service should therefore be unreachable from anywhere else —
loopback, or a private network. The moment it has a public address, the
protection this arrangement provides is one `curl` away from being bypassed.

## Use `Rewrite`, not `Director`

Both fields exist, exactly one may be set, and setting both or neither produces
a `502` on every request rather than an error at startup. Set `Rewrite`.

`Director` is kept for compatibility with the standard package, where it is
deprecated, and the difference is not stylistic. The `Rewrite` path deletes
`Forwarded` and every `X-Forwarded-*` header from the outbound request before
your function runs, so a client cannot smuggle a forged `X-Forwarded-For`
through to the upstream; `SetXForwarded()` then writes the true values.
`Director` — and `NewSingleHostReverseProxy`, which uses it — keeps what the
client sent and appends to it. If the upstream trusts those headers for
anything, that difference is the vulnerability.

## Where the upstream sees the path

`SetURL` joins the target's base path to the incoming path, so with a target
carrying no path the upstream sees the request exactly as the browser sent it —
`/api/users` stays `/api/users`. That is right when the service defines its own
routes under the same prefix.

When the service expects to be at a root it does not have here, strip the mount
point on the way through, adding `net/http` to the imports:

```go
mux.Handle("/api/", http.StripPrefix("/api", proxy))
```

`/api/users` becomes `/users` upstream. Decide this once against the routes the
service actually declares; a mismatch shows up as upstream `404`s that look like
a routing bug in your own application.

## Passing the identity upstream

The service does not authenticate, but it usually still needs to know who is
asking. `Rewrite` runs with the inbound request in hand, and by that point the
framework has resolved the session, so the answer is already in the context.
Replacing the `Rewrite` field above, with
`github.com/shibukawa/popcornwave/pw` added to the imports:

```go
Rewrite: func(r *httputil.ProxyRequest) {
	r.SetURL(apiTarget)
	r.SetXForwarded()
	if auth := pw.RequestAuthentication(r.In.Context()); auth.Authenticated {
		r.Out.Header.Set("X-User", auth.Subject)
	}
},
```

Set the header rather than forwarding whatever arrived under that name, which is
what `Set` does and what makes the value trustworthy: `Rewrite` starts from a
clone of the inbound request, so a client-supplied `X-User` is present on `Out`
until something overwrites it. A header the upstream trusts must be written on
every request, including the unauthenticated ones — an `else` branch calling
`Del` is the safe shape when the upstream treats the header's absence as
anonymous.

## What it will not carry

Protocol upgrades are unsupported and not configurable. A request carrying
`Upgrade`, or an upstream answering `101`, goes to `ErrorHandler` and becomes a
`502` with the default one. WebSockets do not pass through this handler. Neither
do `1xx` responses, so upstream Early Hints never reach the browser.

If the service needs a WebSocket, this is not the right layer for it: put a proxy
that supports upgrades in front of both, and give it
[the configuration a terminating proxy needs](/guides/deployment/reverse-proxy/).
Everything else about this page still applies to the paths that remain HTTP.

Streaming does work. A `text/event-stream` response, or any response of unknown
length, is flushed as it arrives rather than buffered, so server-sent events from
the upstream reach the browser live without setting `FlushInterval`.

## What the framework still applies

Two of the framework's own settings need a decision once traffic is proxied.

**CSRF covers the mount point.** `security.csrf.include` defaults to `/**`, so a
browser `POST` to `/api/` is an unsafe request that needs a token like any other
— read the `pw_csrf` cookie and send `X-CSRF-Token`, using
[the helper from the htmx guide](/guides/interactivity/htmx/#unsafe-requests-and-csrf).
The alternative is `security.csrf.exclude`, and it is only honest when the
upstream authenticates the caller itself; excluding a path in front of a service
that trusts you is removing the check rather than moving it.

**Compression does not apply.** `middleware.compression` encodes what the
framework's own HTML writers produce, and a proxied response is not one of them.
Whatever `Content-Encoding` the upstream sets passes through untouched, which is
the correct outcome — the service already decided.

The keys named here are listed in full in the
[configuration reference](/reference/configuration/).
