---
id: policy:security-response-headers
type: policy
title: Security Response Headers
---
Security header middleware applies one validated browser policy to normal, error, and operational responses.

```yaml
managed:
  content_type_options: X-Content-Type-Options nosniff when enabled
  frame_options: X-Frame-Options DENY or SAMEORIGIN
  referrer_policy: Referrer-Policy configured value
  csp: Content-Security-Policy optional value
  csp_report_only: Content-Security-Policy-Report-Only optional value
  permissions: Permissions-Policy optional value
  hsts: Strict-Transport-Security configured directives
  reporting_endpoints: Reporting-Endpoints naming one endpoint called default, emitted when requirement:browser-report-ingest is enabled
  nel: NEL policy object naming that same endpoint, emitted when requirement:network-error-logging is enabled
rules:
  - set configured headers before downstream response commitment
  - apply to api:error-renderer and policy:operational-endpoints responses, and from slot 52 to the 429 and 503 of SlotRateLimitProcess as well, which slot 60 had left uncovered
  - answer a CORS preflight and stop, per requirement:cors-middleware, which is the one branch of this frame that does not reach the handler
  - send no managed header at all while headers.enabled is false, since the cross-origin half can install this frame on its own and a deployment that turned the headers off must not get them back by admitting an origin
  - emit HSTS only for an effective HTTPS request, resolved through requirement:proxied-request-identity under decision:forwarded-header-trust rather than by this middleware's own copy of the evaluation
  - HSTS preload requires includeSubDomains and at least 31536000 seconds max-age
  - enforced and report-only CSP values may coexist
  - reject carriage return, line feed, and invalid field values at startup
  - omit obsolete X-XSS-Protection
  - append report-to and report-uri to both CSP values while requirement:browser-report-ingest is enabled, and leave a policy that already names either exactly as the author wrote it
boundaries:
  - application authors own CSP sources, nonces, hashes, and Permissions-Policy capabilities
  - CORS is no longer a separate middleware; requirement:cors-middleware is answered by this frame, moved from slot 60 to 52 per decision:cors-above-the-refusals, which supersedes the earlier statement here that it was a concern next door
  - cache policy and Content-Disposition are response-specific application concerns
  - this is not the only source of a CSP header: policy:svg-active-content adds a sandbox policy beside the configured one on an image/svg+xml response, which the browser intersects with this one and can only tighten
```
