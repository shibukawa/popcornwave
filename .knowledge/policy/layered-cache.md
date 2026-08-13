---
id: policy:layered-cache
type: policy
title: Layered Web Cache
---
Data results, component fragments, and eligible full responses have independent cache policies.

```yaml
layers:
  data: function or query ID + normalized arguments + scope + schema version; realized by requirement:data-result-cache, whose identity is declared rather than derived per decision:data-cache-entry-identity
  component: component ID + normalized inputs + scope + build version; realized by requirement:component-output-cache, whose ID is the component plus a digest of its generated plan
  HTTP: URL + representation inputs; owned by whatever sits in front of the process, and the response only declares what it may hold, per policy:component-cache-scope
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
  - coalesce concurrent misses on the data layer, per decision:data-cache-miss-coalescing; the component layer deliberately does not, because a duplicate render costs local CPU where a duplicate fetch costs an upstream call
  - never cache writes or transaction-local reads
  - automatic query caching is limited to analyzable generated reads
  - a cached component cannot declare an api:async-html-value parameter or reach an async record field
  - deployment versions partition keys
  - each layer's policy is independent
```
