---
id: data:route-table
type: data
title: Route Table
---
The exported result of route analysis: every pattern the application registers, every page that produced one, and every path the framework mounts, in one artifact tooling can read.

```yaml
origin: the exported analysis result requirement:httpbinder-extensible-route-analysis lists as a followup, promoted because rule:route-and-template-checks is its first consumer
status: specified, not yet produced; api:cli-doctor reports its checks as not examined until api:cli-generate exports it
producer: api:cli-generate, from the same analysis that emits binders and OpenAPI fragments
entry:
  pattern: the literal method-and-path form as registered
  source: the registration site, as file and position
  handler: the handler identity, when the analysis resolves it
  origin: application registration, generated page, or framework mount
  page: the .pw.html source behind a generated page entry
framework_mounts:
  members: the policy:operational-endpoints health, readiness, and OpenAPI paths, and the requirement:public-asset-delivery mount
  condition: each carries the configuration key that enables it, because a mount that is off collides with nothing
  extensibility: a later framework-owned path joins this list rather than a hard-coded set inside a check
unresolved:
  content: registration calls the analysis could not resolve to a literal pattern, per rule:static-route-discovery
  purpose: a consumer states them as limits instead of reporting a clean table it cannot back up
rules:
  - the table is generated, never hand-written, and stale content is drift like any other generated artifact
  - the table holds what was analyzed, not what will run; a dynamically built pattern is unresolved rather than absent
  - patterns are recorded verbatim so a consumer compares what net/http will compare
consumers:
  - rule:route-and-template-checks
  - the generated OpenAPI document, which already derives from the same analysis
```
