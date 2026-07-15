---
id: decision:thin-httpbinder-integration
type: decision
title: Thin httpbind-go Integration
---
Petitweb wraps httpbind-go tooling and conventions but does not hide its typed calls inside a generic handler abstraction.

```yaml
status: accepted
preserve_in_handler:
  - "httpbinder.Bind[Request](r)"
  - "httpbinder.Write[Response](w, r, response)"
  - "httpbinder.WriteError(w, r, err)"
rationale:
  - system:httpbinder statically discovers request and response types from handler bodies
  - explicit calls keep generated OpenAPI aligned with runtime behavior
  - standard handlers remain independently testable
forbidden:
  - Petitweb-specific route DSL in the MVP
  - reflection-based automatic handler invocation
```
