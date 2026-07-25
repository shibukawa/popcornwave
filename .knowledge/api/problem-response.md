---
id: api:problem-response
type: api
title: Problem Response API
---
Popcorn Wave uses one safe error value for API problems and negotiated HTML error pages.

```yaml
surface:
  - Problem error type re-exported from TinyBind
  - BadRequest(...) Problem
  - NotFound(...) Problem
  - Forbidden(...) Problem
  - InternalServerError(...) Problem
  - WriteProblem(http.ResponseWriter, *http.Request, error)
api:
  format: RFC problem details
html:
  templates:
    400: templates/400.pw.html
    404: templates/404.pw.html
    500: templates/500.pw.html
rules:
  - expose only safe public details
  - retain private causes and diagnostics in logs and traces
  - after response commitment, record failures without rewriting the response
```
