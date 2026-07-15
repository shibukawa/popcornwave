---
id: system:httpbinder
type: system
title: httpbind-go
---
httpbind-go is the authoritative binding, response, validation-error, streaming, and OpenAPI dependency used by Petitweb.

```yaml
module: github.com/shibukawa/httpbind-go
source: https://github.com/shibukawa/httpbind-go
runtime_api:
  bind: "httpbinder.Bind[T](*http.Request) (T, error)"
  write: "httpbinder.Write[T](http.ResponseWriter, *http.Request, T) error"
  write_error: "httpbinder.WriteError(http.ResponseWriter, *http.Request, error)"
  validation: "httpbinder.Validation(...httpbinder.FieldError) error"
  field: "httpbinder.Field(field, location, message) httpbinder.FieldError"
  stream: "httpbinder.NewStream[T](http.ResponseWriter, *http.Request)"
generator:
  package: github.com/shibukawa/httpbind-go/generator
  extensible_analysis: requirement:httpbinder-extensible-route-analysis
  generated_files:
    - httpbinder_gen.go
    - httpbinder_openapi_gen.go
constraints:
  - generator executes with host Go
  - generated mapping path avoids runtime field reflection
  - route discovery analyzes same-package registrations recognized by versioned adapters
```
