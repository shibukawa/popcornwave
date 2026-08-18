---
id: api:logger
type: api
title: Context-Bound Logger API
---
Popcorn Web exposes an immutable logger bound to the context used by api:request-context-accessors.

```yaml
acquire:
  - pw.Logger(context.Context) returns Log, the name handler code uses
  - pwruntime.ReadLogger(context.Context) returns Logger, the same value under its own name
  - naming: the accessor owns the short name in pw, so the type is aliased as Log there
types:
  Level: trace, debug, info, warn, error, or off
  Attribute: data:log-attribute
surface:
  - Logger.Enabled(Level) bool
  - Logger.Trace(message string, attributes ...Attribute)
  - Logger.Debug(message string, attributes ...Attribute)
  - Logger.Info(message string, attributes ...Attribute)
  - Logger.Warn(message string, attributes ...Attribute)
  - Logger.Error(message string, attributes ...Attribute)
  - Logger.Log(context.Context, Level, message string, attributes ...Attribute)
  - Logger.With(attributes ...Attribute) Logger
  - Logger.TraceID() and Logger.SpanID() report the captured correlation
constructors:
  - String, Bool, Int, Int64, Float64
  - Duration renders milliseconds
  - Err renders an error and accepts nil
example: logger.Info("request completed", pw.String("route", "/users"), pw.Int("status", 200))
behavior:
  - acquisition captures the active span context and stable request metadata from the supplied context
  - acquire again from a nested span context to correlate with that child span
  - a missing request capsule returns a stderr fallback logger, never nil, and the zero Logger discards
  - methods are safe for concurrent use and do not mutate the source logger
  - Fatal and Panic methods are omitted because logging must not control process lifecycle
  - log severity does not implicitly change HTTP status, transaction outcome, or span status
  - Log takes a context for cancellation; the severity methods take none, so correlation must be captured at acquisition
request_attributes:
  installer: pwruntime.WithLogAttributes, used by the request ID middleware
  effect: every record taken from the context afterwards carries them without a handler passing them along
implementation:
  backend: decision:slog-handler-log-backend
  sinks: stdout, OTLP, and requirement:local-jsonl-log-capture available only under api:cli-dev, selected by policy:log-emission
  otel: adapt calls to requirement:contrib-otel Logger.Emit with the captured span restored on the context
  stdout: apply policy:log-emission
```
