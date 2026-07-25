---
id: policy:authenticated-path-protection
type: policy
title: Authenticated Path Protection
---
Authentication guard middleware requires data:request-authentication only on configured request paths.

```yaml
decision:
  protected: path matches at least one include and no exclude
  public: path matches no include or matches any exclude
  precedence: exclude overrides include
patterns:
  root: must start with slash
  literal: exact segment
  single: a segment equal to star matches exactly one non-empty segment
  subtree: trailing double-star segment matches zero or more segments
  forbidden: regex, query matching, fragments, and mid-segment wildcards
examples:
  /account: exact path only
  /users/*/settings: one user identifier segment
  /admin/**: /admin and every descendant
evaluation:
  - reject invalid patterns during startup configuration validation
  - match the exact canonical path used by router dispatch
  - ignore query parameters
  - allow a protected request only when data:request-authentication is authenticated
  - otherwise produce the configured redirect or HTTP 401 response without calling the handler
rules:
  - default include is empty; path protection is opt-in
  - exclude is a security-sensitive public override
  - login, callback, bootstrap, and required static paths must remain reachable without this guard
  - authorization by role, tenant, ownership, or permission remains application policy
  - middleware ordering must establish session and authentication before this guard
security:
  - reject malformed escapes and ambiguous path forms before matching
  - prevent encoded separators or dot segments from changing the routed target after matching
  - do not preserve an arbitrary cross-origin return URL for login redirects
```
