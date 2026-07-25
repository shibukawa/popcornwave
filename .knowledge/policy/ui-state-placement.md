---
id: policy:ui-state-placement
type: policy
title: UI State Placement
---
Place state according to its durability and sharing semantics.

```yaml
classification:
  shareable_bookmarkable_history: path or search parameters
  temporary_interaction: client state or flow:partial-refresh
  persistent_mutation: api:server-action
  external_change: dependency invalidation and optional revalidation
rule: search-parameter navigation may update history while refreshing only dependent boundaries
```
