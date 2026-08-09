---
id: requirement:rate-limit-problem-responses
type: requirement
title: Rate Limit Problem Responses
---
api:problem-response represents an enforceable request limit as one safe HTTP 429 response for API and browser clients.

```yaml
standards:
  status: RFC 6585 HTTP 429 Too Many Requests
  retry_after: RFC 9110 Retry-After
  compatibility_headers: X-RateLimit-* are de facto fields, not IETF standards
surface:
  - TooManyRequests(...) Problem
  - RateLimit value carrying limit, remaining, reset, and retryAfter
  - RateLimited(RateLimit, ...) Problem
metadata:
  Retry-After: non-negative delay seconds or HTTP-date; omit when the server cannot recommend a retry time
  X-RateLimit-Limit: non-negative request quota for the described window
  X-RateLimit-Remaining: non-negative requests remaining, normally 0 on the rejecting response
  X-RateLimit-Reset: Unix seconds when the quota window resets
response:
  api: application/problem+json with stable code rate_limit_exceeded
  html: api:error-renderer resolves templates/429.pw.html
  cache: Cache-Control no-store; 429 responses must not be stored
rules:
  - WriteProblem owns header serialization so every representation carries identical retry metadata
  - validate non-negative values, remaining not above limit, reset format, and header safety
  - expose no limiter key, account identifier, internal counter, or policy implementation
  - omission of compatibility metadata remains valid when only a 429 problem is known
  - invalid compatibility metadata is omitted while the safe 429 and no-store policy remain
  - this response contract does not choose client identity, counting scope, storage, or limiting algorithm
documentation: requirement:web-standards-overview
acceptance:
  - JSON and HTML negotiation both preserve status and headers
  - templates/429.pw.html is generated and used like the existing api:problem-response status templates
  - tests cover full metadata, bare 429, HTML negotiation, and invalid metadata
```
