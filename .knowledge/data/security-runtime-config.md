---
id: data:security-runtime-config
type: data
title: Security Runtime Config
---
The `security` binding groups request-forgery protection and browser response-header policy.

```yaml
registration: automatically registered by pw
fields:
  csrf.enabled: bool
  csrf.include: path pattern list
  csrf.exclude: path pattern list
  csrf.form_field: string
  csrf.header: HTTP header name
  csrf.trusted_origins: exact origin list
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
defaults:
  csrf.enabled: false
  csrf.include: ["/**"]
  csrf.exclude: []
  csrf.form_field: _csrf
  csrf.header: X-CSRF-Token
  headers.enabled: true
  headers.content_type_options: true
  headers.frame_options: deny
  headers.referrer_policy: strict-origin-when-cross-origin
  headers.hsts.enabled: false
rules:
  - policy:csrf-protection defines token and request validation
  - policy:security-response-headers defines header behavior
  - reject malformed paths, origins, header names, control characters, and invalid header values at startup
  - redact token material; configured policies may be logged without secret values
```
