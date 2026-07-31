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
  page_tree: api:page-render-runtime is the same kind of boundary for generated page code, and it imports pw rather than being imported by it
public_lower_level:
  - pwpage
  - server
  - middlewares
  - session
  - observability
  - database
  - database/dynamo
  - pwruntime
tinybind:
  role: implementation dependency behind pw
  normal_usage: hidden
  escape_hatch: applications may intentionally import low-level packages such as jsonbind
drivers:
  registry: github.com/shibukawa/popcornwave/database, with one subpackage per engine
  selection: explicit application imports, resolved by rule:rdb-dsn-resolution
  rule: data:project-config never selects runtime drivers; the rdb DSN scheme does
dynamo:
  import: github.com/shibukawa/popcornwave/database/dynamo, per api:dynamo-package
  exception: it is the one store a handler reaches without going through pw, because decision:dynamodb-no-runtime-abstraction found nothing for pw to wrap
  not_an_rdb_engine: it sits beside the engine subpackages and registers nothing into the rule:rdb-dsn-resolution table
  visible_dependency: application code using it names the driver types directly, which the SQL path hides behind database/sql
```
