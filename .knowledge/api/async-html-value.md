---
id: api:async-html-value
type: api
title: Async HTML Value API
---
pw re-exports the htmlbind pending-value surface so a handler starts async work without importing system:tinybind.

```yaml
surface:
  - "pw.Pending[T] alias of htmlbind.Pending[T], the Go form of template `async T`"
  - "pw.Go[T](context.Context, func(context.Context) (T, error)) pw.Pending[T]"
  - "pw.Resolved[T](T) pw.Pending[T]"
  - "pw.Failed[T](error) pw.Pending[T]"
  - pw.AsyncError alias for the presentation-safe recover value
  - pw.UnsetPendingError alias reporting a required async value the caller never set
  - pw.PublicError alias for an error supplying its own safe projection
generated_use: a template `async T` parameter becomes a pw.Pending[T] field in the generated Params struct
rules:
  - the context passed to Go bounds the work; a render bounds only the wait
  - a handle settles once and stays readable, so a wrapper and the leaf may share one value and the work runs once
  - no channel constructor exists; adopt a channel by receiving inside the Go closure
  - a panic inside the work becomes the handle error and reaches the recover clause
  - the zero handle is unset and never blocks or panics
  - an unset handle is legal only where the template declared the awaited type optional
  - a component annotated for policy:layered-cache cannot declare an async parameter or reach an async field
rationale: system:tinybind constrains normal handwritten application code from importing TinyBind directly
```
