---
id: data:security-runtime-config
type: data
title: Security Runtime Config
---
The `security` binding groups request-forgery protection, cross-origin admission, and browser response-header policy.

```yaml
registration: automatically registered by pw
fields:
  csrf.enabled: bool
  csrf.include: path pattern list
  csrf.exclude: path pattern list
  csrf.form_field: string
  csrf.header: HTTP header name
  csrf.trusted_origins: exact origin list
  csrf.anonymous.enabled: bool, whether a visitor with no session is issued a secret at all, per decision:anonymous-csrf-secret-storage; the cookie is signed with the policy:cookie-value-protection keyring rather than a key of its own
  cors.enabled: bool, whether requirement:cors-middleware installs its frame
  cors.include: path pattern list, the grammar of policy:authenticated-path-protection
  cors.exclude: path pattern list
  cors.allowed_origins: exact origin list, or the single literal *
  cors.allow_credentials: bool, which grants a cross-origin read as the logged-in visitor
  cors.allowed_methods: methods admitted in preflight
  cors.allowed_headers: request headers admitted in preflight
  cors.exposed_headers: response headers script may read
  cors.max_age: duration a browser may cache one preflight
  headers.enabled: bool
  headers.content_type_options: bool
  headers.frame_options: deny, sameorigin, or off
  headers.referrer_policy: no-referrer, same-origin, strict-origin, or strict-origin-when-cross-origin
  headers.content_security_policy: optional string
  headers.content_security_policy_report_only: optional string
  headers.permissions_policy: optional string
  headers.hsts.enabled: bool
  headers.hsts.max_age: duration
  headers.hsts.include_subdomains: bool
  headers.hsts.preload: bool
  reporting.enabled: bool, whether requirement:browser-report-ingest serves its endpoint and names it in Reporting-Endpoints
  reporting.path: absolute path inside the reserved framework prefix
  reporting.max_body: bytes accepted from one delivery, separate from server.max_request_body
  reporting.max_reports: reports read from one body
  reporting.rate: records written per second before the remainder is dropped and counted
  reporting.csp_report_uri: bool, whether the legacy directive is appended beside report-to
  reporting.nel.enabled: bool, whether requirement:network-error-logging emits an NEL header
  reporting.nel.max_age: duration the browser keeps the persisted policy
  reporting.nel.include_subdomains: bool
  reporting.nel.success_fraction: fraction of successful requests reported
  reporting.nel.failure_fraction: fraction of failed requests reported
defaults:
  csrf.enabled: false
  csrf.include: ["/**"]
  csrf.exclude: []
  csrf.form_field: _csrf
  csrf.header: X-CSRF-Token
  csrf.anonymous.enabled: false, and turning it on without a configured cookie keyring is a startup failure rather than an unsigned cookie
  cors.enabled: false
  cors.include: ["/**"]
  cors.exclude: []
  cors.allowed_origins: empty, and enabling the frame without one is a startup failure rather than a frame that marks nothing
  cors.allow_credentials: false
  cors.allowed_methods: GET, HEAD and POST, so admitting a write is a stated widening
  cors.allowed_headers: Content-Type and Authorization, the pair the bearer case of requirement:jwt-only-api-authentication needs
  cors.exposed_headers: X-Request-ID, Retry-After and the three X-RateLimit fields, none of them safelisted and all of them written by framework frames
  cors.max_age: 600s, the value Safari caps at
  headers.enabled: true
  headers.content_type_options: true
  headers.frame_options: deny
  headers.referrer_policy: strict-origin-when-cross-origin
  headers.hsts.enabled: false
  reporting.path: /_pw/report
  reporting.max_body: 65536
  reporting.max_reports: 32
  reporting.rate: 10
  reporting.csp_report_uri: true
  reporting.enabled: true, decided 2026-08-10 under requirement:browser-report-ingest
  reporting.nel.enabled: false, because the policy persists in the browser like HSTS
  reporting.nel.max_age: 720h
  reporting.nel.include_subdomains: false
  reporting.nel.success_fraction: 0
  reporting.nel.failure_fraction: 1
rules:
  - policy:csrf-protection defines token and request validation
  - requirement:cors-middleware defines cross-origin admission, its preflight answer, and the Vary it owes; an enabled frame with no origin, allow_credentials with *, and allow_credentials with an include of /** are all startup errors
  - the cors and headers keys configure one frame, per decision:cors-above-the-refusals, so neither can be ordered against the other
  - the OpenAPI document answers a wildcard origin regardless of this binding, per the openapi_document block of requirement:cors-middleware
  - policy:security-response-headers defines header behavior
  - requirement:browser-report-ingest defines the reporting endpoint, and enabling it with headers.enabled false is a startup error because nothing would emit the header
  - reject malformed paths, origins, header names, control characters, and invalid header values at startup
  - redact token material; configured policies may be logged without secret values
```
