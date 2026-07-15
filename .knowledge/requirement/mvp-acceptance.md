---
id: requirement:mvp-acceptance
type: requirement
title: MVP Acceptance Criteria
---
The Petitweb MVP is complete when a new typed service can be scaffolded, generated, tested, run, and compiled with TinyGo through documented commands.

```yaml
criteria:
  - api:cli-init creates concept:project-layout in an empty temporary directory
  - generated starter passes "go test ./..."
  - api:cli-generate is deterministic and its check mode detects drift
  - starter GET /health returns a typed JSON response through system:httpbinder
  - starter POST /echo binds a typed request and writes a typed response through system:httpbinder
  - malformed POST /echo input produces policy:validation-errors problem details
  - POST /echo application validation can report multiple field errors
  - generated OpenAPI contains each discoverable starter route and response schema
  - api:cli-dev serves the starter with host Go
  - api:cli-build produces a TinyGo executable for a supported native target
  - api:cli-check explains missing tools, incompatible versions, and stale generation
quality:
  - CLI command tests cover success, collision, and failure paths
  - generated project smoke test runs outside the Petitweb repository
  - no reflection-based field mapping is introduced
```
