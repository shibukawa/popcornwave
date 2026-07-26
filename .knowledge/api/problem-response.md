---
id: api:problem-response
type: api
title: Problem Response API
---
Popcorn Wave uses one safe error value for API problems and negotiated HTML error pages.

```yaml
surface:
  - Problem error type re-exported from TinyBind
  - FieldError type and Field(field, location, message) FieldError
  - BadRequest(...) Problem
  - Unauthorized(...) Problem
  - Forbidden(...) Problem
  - NotFound(...) Problem
  - Conflict(...) Problem
  - PayloadTooLarge(...) Problem
  - InternalServerError(...) Problem
  - Validation(fields ...FieldError) Problem
  - WriteProblem(http.ResponseWriter, *http.Request, error)
api:
  format: RFC problem details
  field_failures: errors array carrying field, location, and message
html:
  templates:
    400: templates/400.pw.html
    401: templates/401.pw.html
    403: templates/403.pw.html
    404: templates/404.pw.html
    409: templates/409.pw.html
    413: templates/413.pw.html
    500: templates/500.pw.html
rules:
  - expose only safe public details
  - drop field failures from 5xx responses
  - retain private causes and diagnostics in logs and traces
  - after response commitment, record failures without rewriting the response
```
