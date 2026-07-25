---
id: data:runtime-config-registry
type: data
title: Runtime Config Registry
---
The private registry retains every loaded configbind snapshot and field provenance for request-time typed lookup.

```yaml
entry:
  identity:
    - binding prefix
    - generated Go type identity
  value: immutable loaded value
  provenance: source per stable field key
lifecycle:
  - pw.RegisterConfig[T](prefix) registers generated configbind targets before startup load
  - imported plugins may register generated targets before startup load
  - api:runtime-configuration ParseConfig merges defaults, TOML, environment, and CLI for all targets
  - startup validation completes before request dispatch
  - the finalized registry is shared read-only by request capsules
rules:
  - reject duplicate binding identity
  - reject missing generated configbind metadata
  - do not expose registry iteration or mutation to handlers
  - redact secret values while retaining safe source provenance
  - user-defined bindings require no framework schema change
  - plugin-specific fields are unknown unless the owning plugin is imported
```
