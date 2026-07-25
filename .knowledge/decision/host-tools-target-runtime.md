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
    - system:pw-cli
    - system:tinybind generator
    - optional decision:tailwind-host-toolchain
  capabilities:
    - Go AST analysis
    - source generation
    - OpenAPI generation
    - optional static CSS generation
    - flow:public-asset-build precompression
target_phase:
  compiler: TinyGo
  inputs:
    - application source
    - generated binder source
    - generated OpenAPI source when enabled
    - optional generated CSS embedded or served by application code
    - api:cli-init scaffolded public.go and public asset representations
  runtime_rule: rule:tinygo-runtime-compatibility
reason: httpbind-go generator is host-side only
```
