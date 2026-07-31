---
id: policy:csrf-protection
type: policy
title: CSRF Protection
---
CSRF middleware validates a session-bound synchronizer token and request origin on configured unsafe browser requests.

```yaml
scope:
  unsafe_methods: every method except GET, HEAD, and OPTIONS
  protected: canonical path matches security.csrf.include and no security.csrf.exclude
  page_actions:
    requirement: the api:page-action-endpoint prefix must be covered, because those are POST endpoints reachable with ambient credentials and system:tinybind wires no protection around them
    not_yet_scaffolded: this policy has no runtime implementation, so api:cli-init writes no csrf section for a concept:page-tree; the coverage lands with the middleware rather than before it
  patterns: same segment grammar and exclude precedence as policy:authenticated-path-protection
token:
  secret: random value stored only in data:session-record
  request_value: masked token derived for the current session
  sources:
    - configured form field
    - configured HTTP header
  forbidden:
    - URL query
    - request logs
    - error details
validation:
  - require a validated session on a protected unsafe request
  - require constant-time token validation
  - require same-origin Origin or trusted exact origin; use strict Referer fallback only when Origin is absent
  - reject missing, multiple, malformed, expired-session, or mismatched tokens
  - return HTTP 403 through api:error-renderer without calling the application handler
lifecycle:
  - create secret with the session
  - rotate with session creation, login rotation, and privilege changes
  - remove with session revocation
rules:
  - SameSite cookies supplement but do not replace token validation
  - login, OIDC callback, bootstrap, webhook, and non-browser API exclusions are explicit configuration decisions
  - bearer-only requests without cookie authority remain application policy
  - generated templates receive tokens only through api:request-context-accessors
```
