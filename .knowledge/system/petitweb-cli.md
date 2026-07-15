---
id: system:petitweb-cli
type: system
title: Petitweb CLI
---
The Petitweb CLI owns project lifecycle automation while application runtime behavior remains standard net/http plus system:httpbinder.

```yaml
binary: petitweb
commands:
  - api:cli-init
  - api:cli-generate
  - api:cli-dev
  - api:cli-build
  - api:cli-check
configuration: data:project-config
runtime_dependency_policy: decision:thin-httpbinder-integration
execution_split: decision:host-tools-target-runtime
```
