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
rules:
  - set configured headers before downstream response commitment
  - apply to api:error-renderer and policy:operational-endpoints responses
  - emit HSTS only for an effective HTTPS request, resolved through requirement:proxied-request-identity under decision:forwarded-header-trust rather than by this middleware's own copy of the evaluation
  - HSTS preload requires includeSubDomains and at least 31536000 seconds max-age
  - enforced and report-only CSP values may coexist
  - reject carriage return, line feed, and invalid field values at startup
  - omit obsolete X-XSS-Protection
boundaries:
  - application authors own CSP sources, nonces, hashes, and Permissions-Policy capabilities
  - CORS remains a separate middleware concern
  - cache policy and Content-Disposition are response-specific application concerns
```
