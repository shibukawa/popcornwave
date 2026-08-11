---
title: Web Standards
description: The HTTP, security, authentication, error, API, and caching standards Popcorn Wave puts on the wire.
sidebar:
  order: 7
---

A framework can claim to be “standards based” while leaving the consequential
choices to every application. Popcorn Wave takes the narrower position: this
page lists behavior the framework actually emits or enforces, and links to the
guide that owns each detail. An RFC number means a published specification;
`X-RateLimit-*` is called out separately because compatibility is useful, but
does not turn a convention into a standard.

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

## Trace context

W3C Trace Context is a Recommendation rather than an RFC, and the framework is
on both sides of it: `traceparent` and `tracestate` are read off every incoming
request and written onto every request made through an instrumented client, so
a trace continues across a service boundary instead of restarting at it.

The parsing is strict, and what it costs when a header is bad is the point.
A `traceparent` that is malformed, that uses the forbidden `ff` version, that
spells its identifiers in uppercase, or that arrives more than once leaves the
request with no parent — it becomes the root of a new trace and is still served
normally. A `tracestate` that fails the grammar is dropped on its own, keeping
the parent, which is what the specification asks for: the trace still joins up
and only the vendor data is lost.

- [Request tracing](/guides/cross-layer/tracing/#calling-another-service)
- [Telemetry](/guides/architecture/telemetry/#traces-that-cross-services)

## Operational HTTP

Health, readiness, OpenAPI, and API-documentation endpoints have distinct
availability and access rules. Request IDs, body limits, timeouts, recovery,
compression, redirects, and graceful shutdown complete the HTTP boundary around
application handlers.

- [Middlewares](/guides/backend/middlewares/)
- [Operational endpoints](/guides/deployment/operational-endpoints/)
- [Reverse proxies](/guides/deployment/reverse-proxy/)
