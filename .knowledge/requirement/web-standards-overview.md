---
id: requirement:web-standards-overview
type: requirement
title: Web Standards Architecture Overview
---
`website/src/content/docs/guides/architecture/web-standards.md` and its Japanese peer are the headline index of Web standards and interoperable conventions adopted by the framework.

```yaml
audience: application developers evaluating or configuring framework HTTP behavior
page_rule:
  - summarize the framework behavior and its standard or convention in one short entry
  - link each entry to existing detailed documentation instead of repeating configuration or examples
  - distinguish IETF or W3C standards from de facto compatibility conventions
  - claim adopted support only when shipped; label planned support explicitly or omit it
sections:
  security_headers:
    headline: browser response hardening, CSRF, cookie protection, and origin handling
    links:
      - policy:security-response-headers
      - policy:csrf-protection
      - policy:cookie-value-protection
  authentication:
    headline: OIDC, WebAuthn passkeys, Bearer JWT verification, sessions, and assurance challenges
    links:
      - requirement:contrib-oidc
      - requirement:contrib-passkey
      - requirement:jwt-only-api-authentication
      - requirement:session-assurance-levels
  error_responses:
    headline: negotiated RFC problem details, field validation failures, safe HTML pages, and HTTP 429
    links:
      - api:problem-response
      - policy:validation-errors
      - requirement:rate-limit-problem-responses
  api_lifecycle:
    headline: Deprecation, Sunset, migration links, and OpenAPI deprecation metadata
    links:
      - requirement:api-lifecycle-response-headers
  openapi:
    headline: generated operation contracts and configured document or API-reference endpoints
    links:
      - system:tinybind
      - policy:operational-endpoints
      - requirement:dev-api-reference
  cache_control:
    headline: asset validators and immutable caching, private dynamic responses, and no-store protocols
    links:
      - api:public-asset-middleware
      - policy:public-asset-negotiation
      - requirement:navigation-delta-rendering
      - api:live-delivery-protocol
  content_negotiation:
    headline: Accept, content encoding, precompressed assets, and Vary ownership
    links:
      - policy:response-content-encoding
      - policy:public-asset-media-negotiation
      - api:problem-response
  operational_http:
    headline: health, readiness, API documents, redirects, compression, and request limits
    links:
      - policy:operational-endpoints
      - api:redirect-response
      - requirement:response-gzip-encoder
acceptance:
  - every headline links to at least one maintained detail page
  - RFC numbers and standards status are visible where they affect interoperability
  - the index contains no duplicated configuration reference
  - navigation exposes the page under Architecture
```
