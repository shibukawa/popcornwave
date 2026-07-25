---
id: requirement:built-in-config-generation
type: requirement
title: Built-in Config Generation
---
Configuration scaffold generation is a framework lifecycle capability, not application subcommand boilerplate.

```yaml
owner: api:application-lifecycle
activation: framework-owned config-generation CLI option
behavior:
  - api:application-lifecycle Run detects the option after registrations are available
  - merge every framework and application config definition through system:tinybind
  - write the selected TOML or environment scaffold named for the active data:runtime-environment
  - return successfully without database startup, middleware construction, or HTTP listening
errors:
  - invalid format or destination returns before service startup
  - partial destination files are forbidden
application_code:
  remove:
    - GenerateConfigCommand type
    - config generation subcommand registration
    - explicit WriteScaffoldTOML or WriteScaffoldEnv dispatch
```
