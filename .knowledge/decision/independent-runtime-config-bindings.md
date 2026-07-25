---
id: decision:independent-runtime-config-bindings
type: decision
title: Independent Runtime Config Bindings
---
Register each runtime configuration type as an independent named configbind target instead of nesting all settings under one root struct.

```yaml
examples:
  - pw.RegisterConfig[Config]("config")
auto_registered_bindings:
  server: data:server-runtime-config
  session: data:session-runtime-config
  observability: data:observability-runtime-config
  security: data:security-runtime-config
  middleware: data:middleware-runtime-config
optional_registered_bindings:
  auth: data:authentication-runtime-config
  compression: data:compression-runtime-config
rules:
  - one Go type and prefix identify one binding
  - api:runtime-configuration parses every registered binding together
  - user-defined types and prefixes use the same registration and provenance behavior
  - no Popcorn Wave-owned root configuration struct is required
first_release: one configuration load per process
registry: data:runtime-config-registry
```
