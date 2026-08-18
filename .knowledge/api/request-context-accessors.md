---
id: api:request-context-accessors
type: api
title: Request Context Accessors
---
Users retrieve individual framework resources without observing data:request-context-capsule or its context key.

```yaml
shape:
  applied: 2026-08-18; each accessor takes the request handle, and the Context-suffixed form beside it takes context.Context for the layers below the handler
  governed_by: policy:request-scoped-accessor-shape, which also records what deliberately stayed on the context
  second_transport: each base form is registered per requirement:pw-call-registration, so the request argument collapses onto the fasthttp request value rather than refusing the handler; pwfast declares both spellings over one body
surface:
  - pw.Context(r) returns the request's context.Context, and is the supported crossing between the two forms
  - pw.Config[T](r) returns one immutable typed binding
  - pw.Logger(r) returns api:logger bound to the current trace and request
  - pw.DB(r) returns the *sql.DB of the effective group and presence
  - pw.SelectDB pins a group, per api:database-selection
  - generated SQL retrieves the active database or transaction executor
  - session accessors return validated typed session state
  - pw.StartSpan and pw.StartSpanKind open a child of the active span
  - pw.TraceID, pw.SpanID, and pw.Traced report the current correlation
  - security accessors expose validated request security state
rules:
  - accessors accept nil or missing values without unchecked type assertion panic
  - optional resources report absence explicitly
  - config lookup uses the registered generated Go type identity
  - logger lookup never returns nil, and the zero logger discards rather than panicking
  - the request root span is opened by the framework, so a handler starts only spans describing its own work
  - span creation is skipped entirely when nothing exports, because an unexported span is pure cost
  - authorization checks consume authenticated state, never unverified request credentials
  - CSRF token access never exposes the session secret and token values must not be logged
  - generated SQL context functions select the active executor
  - executor and pool lookup resolve the effective group before the executor, per api:database-selection
  - decision:tinybind-sql-runtime consumes the configured generated-code executor resolver
  - api:transaction-runner is the only user-facing way to create a transaction-bearing child capsule
  - callers cannot enumerate or mutate capsule fields
```
