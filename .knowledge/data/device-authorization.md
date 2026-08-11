---
id: data:device-authorization
type: data
title: Device Authorization
---
The device authorization response and its local timing state drive one bounded flow:oidc-device-authorization transaction.

```yaml
wire:
  device_code: required opaque high-entropy string; secret and never shown to the user
  user_code: required short human-readable code
  verification_uri: required absolute user-facing URI
  verification_uri_complete: optional absolute URI carrying the user-code binding
  expires_in: required positive seconds
  interval: optional positive seconds; default 5
local:
  expires_at: derived once from the receipt time and expires_in
  next_poll_at: derived from the effective interval
  effective_interval: interval or 5, increased by slow_down and transport backoff
bounds:
  - reject duplicate, missing, malformed, oversized, zero, negative, or overflowing standard fields
  - reject verification URIs outside the discovered endpoint trust policy
  - cap expiry and interval even when the provider reports larger values
handling:
  - never log, persist by default, or expose device_code through formatting
  - callers may render user_code, verification_uri, and verification_uri_complete
  - a value belongs to one client_id and cannot be reused after terminal success, denial, or expiry
```
