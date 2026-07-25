---
id: api:runtime-configuration
type: api
title: Runtime Configuration API
---
The pw package registers every framework and application configuration type, parses all sources once, and exposes immutable typed values from request context.

```yaml
surface:
  - RegisterConfig[T]("prefix")
  - ParseConfig() error
  - Config[T](context.Context) T
  - Env() string returns the resolved data:runtime-environment token
registration:
  application: RegisterConfig records a generated configbind definition without parsing
  ordering:
    - the generated definition registers during package init
    - RegisterConfig must run after all package init functions, so call it from main
    - calling RegisterConfig from an init function in the owning package panics
  built_in:
    - data:server-runtime-config
    - data:security-runtime-config
    - data:session-runtime-config
    - data:observability-runtime-config
    - data:middleware-runtime-config
parse:
  environment:
    - resolve data:runtime-environment before reading any source
    - select the TOML candidate through policy:config-file-resolution
  scope:
    - all registered framework configuration
    - all registered application configuration
    - CLI configuration
    - registered subcommand input
  called_by:
    - api:application-lifecycle Run when not already parsed
    - api:application-lifecycle Middlewares when not already parsed
  failure: return before request acceptance
observability:
  - log the active environment and the resolved config file path
  - log each effective field and its source
  - mask secret values
source_engine: system:tinybind configbind generation and registry
scaffold: requirement:built-in-config-generation is invoked through api:application-lifecycle
```
