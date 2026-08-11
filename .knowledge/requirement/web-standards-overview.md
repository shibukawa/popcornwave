---
id: requirement:web-standards-overview
type: requirement
title: Web Standards Architecture Overview
---
`website/src/content/docs/appendix/web-standards.md` and its Japanese peer are the headline index of Web standards and interoperable conventions adopted by the framework.

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
    headline: OIDC Authorization Code and Device Authorization, WebAuthn passkeys, Bearer JWT verification, sessions, and assurance challenges
    links:
      - requirement:contrib-oidc
      - requirement:oidc-device-authorization
      - requirement:contrib-passkey
      - requirement:jwt-only-api-authentication
      - requirement:session-assurance-levels
    grant_boundary:
      supported:
        - Authorization Code Flow with mandatory S256 PKCE, state, and nonce
        - RFC 8628 Device Authorization Grant for browserless or input-constrained clients
      unsupported:
        - Resource Owner Password Credentials Grant because OAuth 2.0 Security BCP says it MUST NOT be used
        - Client Credentials Grant because it represents a client without an end-user and belongs to machine-to-machine or batch workloads
      distinction: Client Credentials Grant is not Device Authorization Grant
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
  - OIDC flow support and intentionally unsupported OAuth grants are explicit
  - the index contains no duplicated configuration reference
  - navigation exposes the page under Architecture
```
