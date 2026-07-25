---
id: api:error-renderer
type: api
title: Error Rendering Contract
---
Generated standard error templates implement the HTML branch of api:problem-response while API clients retain RFC problem details.

```yaml
model:
  status: HTTP status
  title: safe summary
  detail: optional safe public detail
  code: stable machine code
  request_id: optional diagnostic identifier
selection:
  html: generated 400, 401, 403, 404, 409, 413, or 500 .pw.html renderer
  api: api:problem-response RFC problem details
fallback:
  - use a minimal safe built-in response when no HTML renderer is registered
  - do not recursively invoke an error renderer when it fails
  - log the renderer failure and return a safe internal response if headers remain uncommitted
rules:
  - generated renderer fragments register during package import
  - renderers receive only sanitized policy:validation-errors output
  - never expose stack traces, SQL, DSNs, tokens, or private error causes
```
