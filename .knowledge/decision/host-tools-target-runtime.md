---
id: decision:host-tools-target-runtime
type: decision
title: Host Tools and Target Runtime Split
---
Code analysis and generation run with standard host Go before application code is compiled with TinyGo.

```yaml
status: accepted
host_phase:
  tools:
    - system:petitweb-cli
    - system:httpbinder generator
  capabilities:
    - Go AST analysis
    - source generation
    - OpenAPI generation
target_phase:
  compiler: TinyGo
  inputs:
    - application source
    - generated binder source
    - generated OpenAPI source when enabled
  runtime_rule: rule:tinygo-runtime-compatibility
reason: httpbind-go generator is host-side only
```
