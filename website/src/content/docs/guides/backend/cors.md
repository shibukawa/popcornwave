---
title: Cross-Origin Requests
description: How to let a browser page served from another origin read this application, and why the answer is different for a bearer API than for a cookie session.
sidebar:
  order: 7
---

```toml
[security.cors]
enabled = true
allowed_origins = ["https://app.example.com"]
```

That is the whole interface for the deployment this exists for: an API here, a
browser application somewhere else, and a bearer token between them.

It is off by default because most deployments serve the pages that read them.
When the origin in the address bar is the origin answering the fetch, the
browser asks nobody's permission and this section is configuration with no
effect. Turn it on when a page you did not serve has to read a response you did.

## What the browser is actually withholding

Without it, a cross-origin fetch does not fail loudly on the server. The request
arrives, the handler runs, the response goes out — and the browser refuses to
hand any of it to the script that asked. Not the body, and **not the status**.

That last part is what makes this worth configuring rather than working around.
Everything this framework does to answer a machine client precisely — the
[typed problem responses](/guides/backend/authentication/), the `401` that means
the token expired, the `429` carrying `Retry-After` — arrives at a cross-origin
caller as one indistinguishable network error. The client cannot tell an expired
token from a rate limit from a server that is down, so it cannot retry
correctly, and the deployment sees nothing at all: its own access log records a
request that worked.

The frame is installed above every part of the stack that can refuse a request,
which is what puts the marking on those refusals rather than only on the
successes. A `429` from the process rate limit, a `403` from the CSRF check, a
`500` from a panic — each one reaches the caller readable.

## The default shape is a bearer API

```toml
[security.cors]
enabled = true
allowed_origins = ["https://app.example.com"]
allowed_methods = ["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"]
```

`allowed_methods` defaults to `GET`, `HEAD` and `POST` — the three a browser
sends without asking first — so widening it to the writing methods is something
you state. `allowed_headers` defaults to `Content-Type` and `Authorization`,
which is what a token-carrying client sends and nothing more.

No cookies are involved here, so nothing about the [CSRF
check](/guides/frontend/security-headers/) applies, and the origins you name are
being granted a read of an API whose every request already proves itself. This
is the configuration to reach for.

## Credentials are a different decision

`allow_credentials = true` lets a listed origin read a response the browser sent
this deployment's cookies with. It is not a wider version of the setting above;
it is a different grant, and it needs its own thought:

```toml
[security.cors]
enabled = true
allowed_origins = ["https://console.example.com"]
allow_credentials = true
include = ["/api/**"]
```

The `include` line is required. With credentials on, an unscoped policy grants
the named origin a read of *everything* the signed-in visitor can see — every
authenticated page, every partial-update fetch, the live region stream — and
startup refuses `include = ["/**"]` for that reason rather than letting a
deployment discover it later.

What it does **not** grant is writes. The CSRF token travels in a cookie only
same-origin script can read, so a page on another origin cannot attach the
header the check wants, and every unsafe request from it is refused with `403`
whatever this section says. If you turn credentials on and expect a cross-origin
`POST` to work, that `403` is where you will land — and the browser will report
it to you as a CORS failure, which sends most people back to edit this section
when the answer is in the other one.

The honest summary: **credentials give a cross-origin reader, not a
cross-origin writer.** If you need writes from another origin, a bearer token
is the mechanism, not a cookie.

## What a refused request looks like from here

A cross-origin failure is famously invisible on the server, so this one is not.
An origin that is not on the list is served normally — nothing is refused, the
browser is what declines — and one record is written naming what did not match:

```
level=INFO msg="cors policy declined a request" reason=origin origin=https://app.example.com path=/api/things
```

`reason` is `origin`, `method`, or `header`. The response never says which,
because the browser was going to answer the question anyway and a precise
refusal only helps a caller map the policy — but the log does, which is the
difference between a five-minute fix and an afternoon.

The records are rate-bounded at ten per second, since the origin is chosen by
whoever is calling; past that they are dropped and the next record that gets
through carries a `dropped` count.

## The OpenAPI document needs nothing

```toml
[server]
openapi = "/openapi.json"
```

The generated document answers `Access-Control-Allow-Origin: *` on its own,
whether or not this section exists. It describes a contract you already chose to
publish, holds nothing that varies per visitor, and is read by tools whose
origins nobody can list in advance — a documentation UI you host elsewhere, a
client generator, a linter in CI. A wildcard forbids credentials, so a document
you put behind authentication still answers a cross-origin reader with the
unauthenticated response and discloses nothing.

## Caching

While an origin list is configured, every in-scope response carries `Vary:
Origin` — including responses this policy left unmarked, which looks redundant
and is not. The decision read the `Origin` header even when the answer was to
write no header at all, and a shared cache that keyed on nothing would hand one
origin's grant to a caller who was refused it.

The one exception is `allowed_origins = ["*"]`, which answers identically for
every caller and therefore carries no `Vary` and stays fully cacheable. That
form is available only with credentials off, which the specification requires
and startup enforces.

## When not to use it

Three cases, and they cover most of the deployments that reach for this page.

**Your pages and your API share an origin.** Nothing here applies. Adding the
section changes no response.

**Something in front of the application already does it.** A CDN or ingress with
its own CORS configuration is a fine place for this, and two layers writing the
same header is a conflict, not a redundancy. Pick one. The reason this exists
in the framework is that `pw dev` has no such layer, so a developer building the
cross-origin client meets the failure locally with nothing to configure.

**The caller is not a browser.** A server, a CLI, a job runner, a mobile app's
native HTTP client — none of them enforces any of this. CORS is a browser
restriction, and configuring it for a client that ignores it buys nothing.

There is a fourth case worth naming because it looks like this problem and is
not: a second hostname in front of one application. If what you want is an
existing API reachable from your own pages, mounting it inside this application
avoids the whole subject — see the [service proxy
guide](/guides/backend/service-proxy/).

## Where the WebSocket sits

Not here. A WebSocket upgrade is not subject to CORS at all; it carries cookies
across origins with no preflight and no `Access-Control-Allow-Origin`, and its
defence is the origin check described in the [WebSocket
guide](/guides/backend/websocket/). Admitting an origin in this section grants it
nothing on that path, and refusing one denies it nothing.

Every key is listed in the [configuration
reference](/reference/configuration/#securitycors).
