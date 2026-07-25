---
id: system:pw-cli
type: system
title: pw CLI
---
The pw CLI owns project bootstrap, generation, development, and builds while runtime behavior remains in the application and standard net/http-compatible packages.

```yaml
binary: pw
commands:
  - api:cli-init
  - api:cli-generate
  - api:cli-dev
  - api:cli-build
  - api:cli-schema-init
  - api:cli-seed
configuration: data:project-config
runtime_dependency_policy: concept:public-package-boundaries
execution_split: decision:host-tools-target-runtime
initial_exclusions:
  - no pw test command; use go test ./...
  - no standalone pw check command; use pw generate --check for generated drift
```
