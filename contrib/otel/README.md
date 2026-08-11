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

## Continuing the trace in the service you call

An incoming `traceparent` is picked up by the server middleware. To keep the
trace going the other way, give the outbound client an instrumented transport:

```go
client := otelhttp.NewClient(http.DefaultClient)

// Pass the request context. The client span becomes the parent the callee
// adopts, so its work shows up under this call rather than beside it.
request, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
response, err := client.Do(request)
```

`otelhttp.NewTransport` is the same thing for a client you build yourself. Both
open the client span and write the header in one place, because the header names
the span the callee parents onto: injecting anywhere else names the span above
it, and the callee's work comes back as a sibling of the call that caused it.

Do not hand an instrumented client to the exporter. `otlphttp.New` removes the
instrumentation from the client it is given for that reason — exporting a span
would open a span, whose export would open another. Anything else on the
transport chain, such as an authenticating or retrying wrapper, is left alone.

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
