---
id: flow:error-template-generation
type: flow
title: Error Template Generation
---
The host generator converts standard .pw.html error pages into TinyGo-safe renderers used by api:problem-response.

```yaml
trigger: api:cli-generate finds templates/400.pw.html, 404.pw.html, or 500.pw.html
steps:
  - parse and escape-analyze through requirement:contrib-html-template
  - bind the framework safe error-page model
  - reject unsupported fields, helpers, inclusions, or output contexts
  - emit a package-level renderer beside each source
  - format and atomically update policy:generated-artifacts
rules:
  - generated fragments register during package import
  - source HTML remains the human-owned presentation artifact
```
