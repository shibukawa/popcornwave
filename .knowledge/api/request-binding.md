---
id: api:request-binding
type: api
title: Request Binding API
---
Handlers call pw.Parse with a typed input and receive generated request binding without importing TinyBind.

```yaml
surface:
  - Parse[T](*http.Request) (T, error)
generator:
  discovery: api:cli-generate scans Parse[T] call sites
  mapping: system:tinybind performs generated binding and validation
inputs:
  - path parameters
  - query parameters
  - headers
  - cookies
  - forms and multipart forms
  - JSON bodies
rules:
  - no runtime reflection requirement on TinyGo targets
  - binding errors implement the structured problem contract
```
