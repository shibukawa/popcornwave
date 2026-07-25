---
id: rule:tinygo-runtime-compatibility
type: rule
title: TinyGo Runtime Compatibility
---
Release builds must prove that application runtime code and generated mapping compile with the selected TinyGo toolchain.

```yaml
requirements:
  - installed TinyGo satisfies decision:tinygo-042-baseline
  - tinygo executable is available
  - host Go version is supported by the installed TinyGo version
  - generated artifacts are current
  - application runtime does not import generator packages
  - no runtime reflection-based request mapping
  - configured target supports the application's net/http listener model
baseline_evidence:
  httpbind_go_verified_tinygo: 0.40.1
  compatible_go_range_for_that_baseline: 1.19-1.25
  source: https://github.com/shibukawa/httpbind-go#tinygo
verification: api:cli-build and package target tests
```
