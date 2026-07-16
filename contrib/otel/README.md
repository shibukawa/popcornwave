# Minimal OpenTelemetry

`contrib/otel` is an experimental, TinyGo-friendly subset for manual traces,
correlated or standalone logs, W3C Trace Context, and OTLP/HTTP JSON. It records
all spans and intentionally leaves sampling to the relay or collector. Metrics,
baggage, gRPC, auto-instrumentation, and API compatibility with the upstream Go
SDK are not provided.

```go
exporter, err := otlphttp.NewFromEnv()
if err != nil { return err }

resource := otlphttp.ResourceFromEnv()
traces := trace.NewProvider(
    trace.NewBatchProcessor(exporter, trace.BatchConfig{}),
    trace.WithResourceAttributes(resource...),
)
logs := otellog.NewProvider(
    otellog.NewBatchProcessor(exporter, otellog.BatchConfig{}),
    otellog.WithResourceAttributes(resource...),
)
trace.SetDefaultProvider(traces)
otellog.SetDefaultProvider(logs)

handler := middlewares.Otel()(mux)

// In a handler, IDs and child spans come from the request context.
traceID := middlewares.TraceID(r.Context())
ctx, span := middlewares.StartSpan(r.Context(), "load-user",
    otel.String("db.system.name", "postgresql"))
defer span.End()

// ctx correlates the log; context.Background() produces a valid standalone log.
logs.Logger("example").Emit(ctx, otellog.Record{
    Severity: otellog.SeverityInfo,
    Body: "user loaded",
})
_ = traceID
```

Call both providers' `Shutdown` with a deadline during application shutdown.
The batch processors use bounded queues, never block producers when full, and
report dropped counts with `Dropped()` and final export failures with `Error()`.

Supported environment variables:

- `OTEL_EXPORTER_OTLP_ENDPOINT`
- `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`
- `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT`
- `OTEL_EXPORTER_OTLP_HEADERS`
- `OTEL_EXPORTER_OTLP_TIMEOUT` (milliseconds)
- `OTEL_SERVICE_NAME`

Signal-specific endpoint variables are complete URLs. The common endpoint is a
base URL to which `/v1/traces` and `/v1/logs` are appended. Static headers are
copied and never included in exporter errors. With no endpoint variable, the
secure default is `https://localhost:4318`.
