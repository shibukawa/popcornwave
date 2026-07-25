---
id: data:ui-dependency-graph
type: data
title: UI Dependency Graph
---
Generation records the inputs and scopes that can change each server-component output.

```yaml
sources:
  - component inputs
  - api:typed-external-function calls
  - declared data tags
  - URL and search parameters
  - session and authorization scope
  - parent and child boundaries
correctness:
  input_hash: execution optimization
  dependency_invalidation: freshness authority
  rationale: output may vary with database state, permissions, locale, or time
```
