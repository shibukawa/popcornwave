---
id: policy:validation-errors
type: policy
title: Validation and Error Policy
---
All request conversion and application validation failures use httpbind-go HTTP errors and RFC 9457 problem responses.

```yaml
conversion_errors:
  producer: generated system:httpbinder binder
  constructor: httpbinder.BindError
business_validation:
  constructor: httpbinder.Validation
  field_constructor: httpbinder.Field
  collect: all independently detectable field failures before returning
application_errors:
  constructors:
    - httpbinder.BadRequest
    - httpbinder.Unauthorized
    - httpbinder.Forbidden
    - httpbinder.NotFound
    - httpbinder.Conflict
    - httpbinder.Internal
response:
  writer: httpbinder.WriteError
  media_type: application/problem+json
  field_shape:
    - field
    - location
    - message
security:
  - hide internal causes for 5xx responses
  - use stable machine-readable problem codes
```
