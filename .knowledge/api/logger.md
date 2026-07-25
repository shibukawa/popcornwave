---
id: api:logger
type: api
title: Context-Bound Logger API
---
Popcorn Wave exposes an immutable logger bound to the context used by api:request-context-accessors.

```yaml
acquire: ReadLogger(context.Context) Logger
types:
  Level: trace, debug, info, warn, or error
  Attribute: data:log-attribute
surface:
  - Logger.Enabled(Level) bool
  - Logger.Trace(message string, attributes ...Attribute)
  - Logger.Debug(message string, attributes ...Attribute)
  - Logger.Info(message string, attributes ...Attribute)
  - Logger.Warn(message string, attributes ...Attribute)
  - Logger.Error(message string, attributes ...Attribute)
  - Logger.With(attributes ...Attribute) Logger
example: logger.Info("request completed", String("route", "/users"), Int64("status", 200))
behavior:
  - acquisition captures the active span context and stable request metadata from the supplied context
  - acquire again from a nested span context to correlate with that child span
  - a missing request capsule returns the configured process logger or a safe no-op logger, never nil
  - methods are safe for concurrent use and do not mutate the source logger
  - Fatal and Panic methods are omitted because logging must not control process lifecycle
  - log severity does not implicitly change HTTP status, transaction outcome, or span status
implementation:
  otel: adapt calls to requirement:contrib-otel Logger.Emit with the captured context
  stdout: apply policy:log-emission
```
