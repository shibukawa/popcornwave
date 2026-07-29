---
id: concept:public-package-boundaries
type: concept
title: Public Package Boundaries
---
Popcorn Wave presents one compact handwritten API while keeping generated and advanced lower-level integrations inspectable.

```yaml
handwritten_default:
  import: github.com/shibukawa/popcornwave/pw
  rule: normal application handlers and entry points use pw
generated_runtime:
  import: github.com/shibukawa/popcornwave/pwruntime
  rule: generated code may use pwruntime but handwritten application code normally does not
public_lower_level:
  - server
  - middlewares
  - session
  - observability
  - database
  - pwruntime
tinybind:
  role: implementation dependency behind pw
  normal_usage: hidden
  escape_hatch: applications may intentionally import low-level packages such as jsonbind
drivers:
  registry: github.com/shibukawa/popcornwave/database, with one subpackage per engine
  selection: explicit application imports, resolved by rule:rdb-dsn-resolution
  rule: data:project-config never selects runtime drivers; the rdb DSN scheme does
```
