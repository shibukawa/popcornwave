---
id: data:loaded-configuration
type: data
title: Loaded Configuration
---
Each independently registered configbind target is merged and validated at startup, then exposed as an immutable typed snapshot with per-field provenance.

```yaml
registration:
  api: pw.RegisterConfig[T](prefix)
  model: decision:independent-runtime-config-bindings
  identity: binding prefix plus generated Go type
precedence:
  - typed defaults
  - TOML
  - environment variables
  - CLI arguments
toml_selection: policy:config-file-resolution using data:runtime-environment
mapping:
  reflection: forbidden
  mechanism: reuse generated JSON-to-struct mapping
generated_bindings:
  - environment variables
  - CLI arguments
access:
  - pw.Config[T](context.Context) returns T
  - internal provenance remains available for startup logging
registry: data:runtime-config-registry
request_storage: data:request-context-capsule
```
