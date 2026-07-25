---
id: policy:layered-cache
type: policy
title: Layered Web Cache
---
Data results, component fragments, and eligible full responses have independent cache policies.

```yaml
layers:
  data: function or query ID + normalized arguments + scope + schema version
  component: component ID + normalized inputs + scope + build version
  HTTP: URL + representation inputs
defaults:
  safe_generated_reads: cached
  deterministic_server_components: cached
  policy: application TTL and stale window unless opted out
declarations:
  - default shared
  - explicit TTL
  - private scope
  - no-cache
rules:
  - canonical keys contain every explicit argument
  - private keys contain user, session, or tenant scope
  - input hash controls execution reuse; output hash controls transfer
  - expiry and tag invalidation are supported
  - configured expensive reads may use stale-while-revalidate
  - coalesce concurrent misses
  - never cache writes or transaction-local reads
  - automatic query caching is limited to analyzable generated reads
  - deployment versions partition keys
  - each layer's policy is independent
```
