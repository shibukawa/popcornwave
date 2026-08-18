---
id: policy:request-scoped-accessor-shape
type: policy
title: Request-Scoped Accessor Shape
---
A framework accessor a handler calls takes the request handle; the same accessor for code below the handler takes context.Context and carries a Context suffix.

```yaml
serves: decision:transport-handle-containment
naming:
  base: pw.DB(r), pw.Config[T](r), pw.Logger(r), pw.RequestAuthentication(r)
  context_form: pw.DBContext(ctx), pw.ConfigContext[T](ctx), pw.LoggerContext(ctx)
  precedent: database/sql spells the same pair Exec and ExecContext, so the suffix reads as Go rather than as this framework
  canonical: the base form in scaffolds, examples, tutorial, and reference; the Context form documented as what code below the handler takes
why_both_survive:
  - generated SQL takes context.Context and is owned by system:tinybind, per decision:tinybind-sql-runtime
  - business logic below the handler stays callable without a request, which is what makes it testable
  - so context.Context remains the currency below the handler, the request handle becomes the currency at it, and the framework owns the single conversion between them
the_conversion:
  surface: pw.Context(r) is the supported way to obtain the request's context.Context
  ports: a backend whose request value already is a context.Context returns it unchanged
  today: r.Context() yields the same value and stays legal, reported at note severity by rule:transport-handle-checks
  why_it_is_not_only_cosmetic: r.Context() is a method on the net/http type, so a handler calling it names that type and takes the decision:transport-compatibility-fallback path; pw.Context is the same value without that consequence
handle_spelling:
  resolved: the concrete net/http request pointer type, with no alias and no interface
  reason: decision:transport-source-transform rewrites the call sites for another backend, so the signature never has to satisfy two backends at once
  rejected:
    alias: a build-tagged pw.Request, which the transform makes unnecessary
    interface: one carrying a Context method, which would accept any type having one and would pay dispatch on every accessor call
  helps_the_transform: an accessor taking r in a fixed position is a call the transform rewrites mechanically, where r.Context() threaded into an arbitrary expression is one it must reason about
pinning_returns_a_context:
  resolved: api:database-selection SelectDB takes r and returns context.Context, which is the same signature shape as pw.Context and not an exception to it
  why_it_ports: a backend whose request value implements context.Context is a valid parent for context.WithValue, so deriving a context needs nothing the backend does not already provide
  earlier_error: this was recorded as impossible on the reasoning that a pooled backend has one request value and nothing to derive; that confuses deriving a request with deriving a context, and only the first is blocked
  what_is_actually_blocked: returning a derived request, the r.WithContext shape, because a second request value is what a pooled backend cannot produce
  what_it_preserves: the pin stays a property of a value, so every api:database-selection rule about effective group, child contexts, and transaction scope survives unchanged
  reading:
    unpinned: pw.Context(r) for the context generated SQL takes
    pinned: pw.SelectDB(r, group) for the same context with a group pinned
    shape: both are request in, context out, which is the boundary crossing rather than two conventions
  costs_on_a_pooled_backend:
    allocation: context.WithValue allocates a wrapper where the framework's own request state would otherwise be a user value on the request; it is one allocation on the path that pins, not on every request
    lifetime: the wrapper holds the pooled request value, so a context derived from the request inherits the rule that it does not outlive the handler
  transaction: api:transaction-runner keeps taking a context, because a caller holding a pinned context wants the transaction on that group; its request form is the entry point and its Context form is what a pinned caller uses
migration:
  applied: 2026-08-18, in one change rather than the three steps below, because the rename is what makes the base name available and splitting it would have shipped two spellings of the same call
  fact: every accessor took ctx first, so the base form was a breaking rename across a pre-1.0 surface
  order: add the base form, move scaffolds and examples onto it, then rename the ctx form with its suffix
  not_removed: no context form is deleted, because the layers below the handler are its callers
  what_moved: Config, Logger, DB, DBDriver, SelectDB, SelectWriteDB, SelectSessionDB, Transaction, RequestAuthentication, Authenticated, StartSpan, StartSpanKind, TraceID, SpanID, Traced, MemoStore, LocalePath, and the locale read, plus pw.Context itself
  the_one_exception: the locale read has no base name to take, Locale being the type; RequestLocale is the base form and LocaleContext, which already carried the suffix, is unchanged
  stayed_on_the_context: the cache operations, Go, WithLogAttributes, and the startup entries, none of which reads the request — each passes its context down rather than reading a value out of it
  a_call_site_migrates_by_argument: pw.DB(ctx) becomes pw.DB(r), so the diff of an application is r for r.Context() and never a new name
rules:
  - an accessor pair reads one resource; the two forms never diverge in behavior
  - a new request-scoped accessor lands with both forms, the way policy:context-value-storage requires a capsule field and its accessor together
  - a base form may return a context.Context, and never returns a derived request
  - a context derived from the request is bound by the same lifetime rule as the request, per decision:transport-handle-containment
  - api:request-context-accessors is the surface list this shape applies to
```
