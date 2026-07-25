---
id: policy:context-value-storage
type: policy
title: Context Value Storage Policy
---
Minimize context.WithValue chain depth while preserving values whose nesting and shadowing carry semantics.

```yaml
bundled_in: data:request-context-capsule
bundled:
  - database pool
  - active database transaction
  - data:runtime-config-registry with all configbind values and provenance
  - request root span
  - api:logger backend and stable request logging attributes
  - validated data:session-record safe view
  - verified data:request-authentication
  - masked CSRF request token; never the stored session secret
  - future stable request resources approved by this contract
individual_values:
  - active OpenTelemetry span context created by each nested Tracer.Start
  - values whose parent-child shadowing is their required behavior
unchanged_context_mechanisms:
  - cancellation
  - deadline
rules:
  - one capsule context value replaces one value per flat framework resource
  - do not bundle nested active span state
  - do not expose the capsule as an application dependency container
  - add a field and api:request-context-accessors entry together
  - api:transaction-runner may derive one scoped child capsule for an explicit transaction
rationale: context.Value lookup traverses the context chain, so independent flat values add cumulative lookup depth
```
