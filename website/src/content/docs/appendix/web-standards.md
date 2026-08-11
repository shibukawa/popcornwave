---
title: Web Standards
description: The HTTP, security, authentication, error, API, and caching standards Popcorn Wave puts on the wire.
sidebar:
  order: 3
---

A framework can claim to be “standards based” while leaving the consequential
choices to every application. Popcorn Wave takes the narrower position: this
page lists behavior the framework actually emits or enforces, and links to the
guide that owns each detail. An RFC number means a published specification;
`X-RateLimit-*` is called out separately because compatibility is useful, but
does not turn a convention into a standard. Publication is not the only test that
matters either. A specification one browser engine ships is a different
proposition from one they all ship, and one that every server can send but no
browser will read is a different proposition again. The last two sections cover
features declined on those grounds.

## Security headers and browser boundaries

The security middleware owns CSP, HSTS, `X-Content-Type-Options`,
`X-Frame-Options`, `Referrer-Policy`, and `Permissions-Policy`. CSRF protection
combines a session-bound token with an origin check; cookie policy controls
`Secure`, `HttpOnly`, `SameSite`, signing, and encryption.

- [Security response headers](/guides/frontend/security-headers/)
- [CSRF and deployment security](/guides/architecture/security/)
- [Cookie protection](/guides/backend/cookies/)

## Authentication

Browser authentication uses OpenID Connect and WebAuthn passkeys. API-only
applications can verify Bearer JWTs, while session and assurance policy keeps
the resulting identity available through one request contract.

- [Authentication](/guides/backend/authentication/)
- [Authentication design](/guides/backend/authentication-design/)
- [Sessions](/guides/backend/sessions/)

## Error responses and rate limits

`pw.WriteProblem` negotiates RFC 9457 Problem Details or a safe HTML error page.
HTTP 429 follows RFC 6585 and can carry the standard `Retry-After` field.
`X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `X-RateLimit-Reset` are also
available for clients that use the established compatibility convention; they
are not IETF-standard fields.

- [Responses and problem details](/guides/frontend/responses/#errors)
- [Runtime error API](/reference/runtime/#errors)

## API lifecycle

Deprecation and shutdown are related, but they do not say the same thing.
RFC 9745 `Deprecation` says when a resource becomes discouraged while leaving
its behavior intact. RFC 8594 `Sunset` says when the resource is expected to
become unavailable. `pw.LifecycleHeaders` accepts both dates in one value,
validates their order, and serializes each in the format its RFC requires.

```go
lifecycle, err := pw.LifecycleHeaders(pw.Lifecycle{
	DeprecatedAt:     time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	SunsetAt:         time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
	DocumentationURL: "https://example.com/migrations/v2",
})
if err != nil {
	log.Fatal(err)
}
router.Handle("GET /v1/items", lifecycle(http.HandlerFunc(listV1Items)))
```

The response then carries a Structured Field Date for `Deprecation`, an
HTTP-date for `Sunset`, and a `Link` relation for the migration document. The
middleware only announces lifecycle state; it neither changes the response nor
turns the route off.

## OpenAPI

`pw generate` derives OpenAPI 3.1 operations from registered handlers, request
binding, response writers, streams, and problem constructors. The generated
document and optional Scalar or Swagger UI are served as operational endpoints,
behind the same path guard as application routes.

- [API documentation](/productivity/api-documentation/)
- [Handlers and generated contracts](/guides/frontend/handlers/)
- [Operational endpoints](/guides/deployment/operational-endpoints/)

## Caching and content negotiation

HTML is private and `no-store` unless its document shell declares a public
scope. Fingerprinted assets use validators and immutable caching; navigation
deltas and live delivery use `no-store`. Content encoding and precompressed
asset selection maintain the corresponding `Vary` fields, while 429 responses
are never stored.

- [Rendering cache](/guides/frontend/rendering-cache/)
- [Static assets](/guides/frontend/static-assets/)
- [Compression](/guides/frontend/compression/)
- [Responses](/guides/frontend/responses/)

## Operational HTTP

Health, readiness, OpenAPI, and API-documentation endpoints have distinct
availability and access rules. Request IDs, body limits, timeouts, recovery,
compression, redirects, and graceful shutdown complete the HTTP boundary around
application handlers.

- [Middlewares](/guides/backend/middlewares/)
- [Operational endpoints](/guides/deployment/operational-endpoints/)
- [Reverse proxies](/guides/deployment/reverse-proxy/)

## Client hints, and why they stay out

`Accept-CH` looks like the right way to render a page the reader already wants.
The server advertises the hints it cares about, the browser sends
`Sec-CH-Prefers-Color-Scheme` on the next request, and a reader who prefers dark
gets a dark first paint with no script and no flash. Popcorn Wave sends no
`Accept-CH` and reads no `Sec-CH-*` request header, because the mechanism has one
engine behind it.

| Header | Chromium | Firefox | Safari |
| --- | --- | --- | --- |
| `Sec-CH-Prefers-Color-Scheme` | 93 | — | — |
| `Sec-CH-Prefers-Reduced-Motion` | 108 | — | — |
| `Sec-CH-Viewport-Width` | 97 | — | — |
| `Critical-CH` | 91 | — | — |

Every one of these is still marked experimental, and `Critical-CH` — the header
that makes a browser retry the first navigation so the hints arrive in time — is
not on the standards track at all. An application built on them renders correctly
in Chrome and Edge and falls back to a guess everywhere else. A framework helper
would not close that gap; it would only make the narrow path look like the
supported one.

The alternatives are also better, which is what settles it. `prefers-color-scheme`
in CSS answers the operating-system preference before first paint, in every
browser, with no round trip and no cache cost. That leaves the hint improving one
case — an explicit override the reader chose on this site — and a cookie carries
that choice everywhere while the hint carries it in one engine. Layout that
depends on viewport width is worse still. The width changes when the reader
rotates the phone, with no new navigation to correct it, so a server-side
decision goes stale while it is still on screen; container queries and `srcset`
decide per element and stay correct.

Caching closes the argument. A response that varies on the reader's color scheme
has two representations, which is affordable. One that varies on viewport width
has a representation per distinct window size, which is not — a shared cache
would miss on nearly every request, and the page would lose more to that than the
hint ever returned.

None of this is enforced. A handler can read the header itself and set `Vary` to
match, and an application that has measured the flash and accepts the cache cost
is free to do exactly that. It is a choice the application owns, with reasoning
the framework declines to make on its behalf.

## Server-Timing trailers, and why they stay out

`pw dev` already measures the work behind a response. Tracing opens a span for
the render, one for each settled boundary, and one per statement, so the
breakdown of a slow page exists before anyone goes looking for it. Returning it
on the same response is the obvious next step, and HTTP has the mechanism for it:
`Trailer: Server-Timing` carries fields that are known only after the body, which
on a streamed page is most of what is worth knowing. DevTools would show them
beside the request. Popcorn Wave sends no trailer, because on the connection
`pw dev` serves, nothing reads one.

| | Chromium | Firefox | Safari |
| --- | --- | --- | --- |
| `Server-Timing` as a response header | ✓ | ✓ | ✓ |
| `Server-Timing` as a trailer | — | HTTPS only | — |
| any trailer, read from `fetch()` | — | — | — |

Firefox is the only engine that has ever exposed a trailer in any form. It
restricted that support to HTTPS, and it advertises trailer support only over
HTTP/2, which it speaks only to `https://` origins. Chromium has declined
trailers repeatedly, on the grounds that too much of a browser interposes on
network requests for the change to stay contained. `pw dev` binds a cleartext
HTTP/1.1 port. Chrome therefore ignores the trailer, Firefox refuses it for not
being HTTPS, and the panel the developer opened it for stays empty.

The third row is why nothing routes around that. `fetch()` cannot read trailers
at all; DevTools can, and only for `Server-Timing`, as a special case wired into
the network panel. No script on the page and no pane in the dev console can pick
a trailer up and render it. One consumer remains, and two engines out of three
never populate it.

Forcing chunked encoding in development was the other half of the idea, and it
falls with the trailer it existed to carry. It would have cost something on its
own account. A development server that frames its responses differently from the
deployed one stops rehearsing it, and this framework has already paid for that
once: a boundary-marker bug that never appeared in development surfaced in
production as soon as a proxy, a TLS record, or a compressing encoder split the
bytes where development never had.

None of the timing is lost, which is what settles it. Those spans are in the
[development telemetry viewer](/productivity/dev-telemetry-viewer/) as a tree,
correlated with the log records and statements from the same request — including
everything that happened after the response committed. That is the part no
response header can carry, and the part the trailer was proposed for.

Nothing here is enforced either. A handler can set `Server-Timing` as an ordinary
response header and get a time-to-first-byte breakdown in every browser's
DevTools, and it can write a trailer beside it. What it cannot do is expect the
trailer to be read.
