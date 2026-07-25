---
id: policy:validation-errors
type: policy
title: Validation and Error Policy
---
All request conversion and application validation failures use api:problem-response values and safe RFC problem responses.

```yaml
conversion_errors:
  producer: generated api:request-binding binder
business_validation:
  representation: problem details with field failures
  collect: all independently detectable field failures before returning
application_errors:
  constructors:
    - pw.BadRequest
    - pw.NotFound
    - pw.Forbidden
    - pw.InternalServerError
response:
  writer: api:problem-response WriteProblem
  media_type: application/problem+json
  field_shape:
    - field
    - location
    - message
security:
  - hide internal causes for 5xx responses
  - use stable machine-readable problem codes
  - HTML negotiation uses generated 400, 404, and 500 templates when possible
```
