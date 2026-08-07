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
  - api:cli-prepare
  - api:cli-migrate
  - api:cli-seed
  - api:cli-doctor
  - api:cli-version
  - api:cli-fmt
  - api:cli-lsp, proposed by decision:language-server-in-pw-cli
configuration: data:project-config
runtime_dependency_policy: concept:public-package-boundaries
execution_split: decision:host-tools-target-runtime
distribution: requirement:cli-distribution
initial_exclusions:
  - no pw schema-init command; requirement:database-migration replaced it
  - no pw test command; use go test ./...
  - no standalone pw check command; pw generate --check answers generated drift and api:cli-doctor answers configuration and wiring
```
