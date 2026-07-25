---
id: requirement:typed-http-contract
type: requirement
title: Typed HTTP Contract
---
Generated mappings cover request inputs, typed HTML and API responses, application problems, OpenAPI fragments, SQL, and enabled streaming types through the pw surface.

```yaml
inputs:
  - route parameters
  - query parameters
  - headers
  - cookies
  - forms and multipart forms
  - JSON bodies
outputs:
  - api:html-response
  - api:api-response
  - api:problem-response
  - deterministic assembled OpenAPI JSON and YAML
  - api:typed-stream
database: flow:sql-generation
errors: policy:validation-errors
```
