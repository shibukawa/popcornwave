package pw

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/shibukawa/popcornwave/contrib/otel"
	"github.com/shibukawa/popcornwave/contrib/otel/exporter/otlphttp"
	otellog "github.com/shibukawa/popcornwave/contrib/otel/log"
	"github.com/shibukawa/popcornwave/contrib/otel/trace"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// Stdout record formats.
const (
	StdoutFormatJSON      = "json"
	StdoutFormatPlaintext = "plaintext"
)

// observability is the resolved emission and tracing state of one process.
type observability struct {
	backend *pwruntime.LogBackend
	traces  *trace.Provider
	logs    *otellog.Provider
	// tracing reports whether a request root span should be created. It is
	// false when nothing exports, because an unexported span is pure cost.
	tracing bool
}

// buildObservability resolves configuration and the standard OTLP environment
// into one emission policy.
//
// The environment matters as much as the file here: api:cli-dev injects
// OTEL_EXPORTER_OTLP_ENDPOINT into the process it starts, so a developer gets
// traces and correlated logs without writing configuration at all.
func buildObservability(config ObservabilityConfig, env string) (*observability, error) {
	minimum, err := parseLevel(config.MinimumLevel, LevelInfo)
	if err != nil {
		return nil, fmt.Errorf("observability.minimum_level: %w", err)
	}
	resolved := &observability{}
	endpoint := otlpEndpoint(config)
	if config.Otel.Enabled || endpoint != "" {
		if err := resolved.startOtel(config, endpoint); err != nil {
			return nil, err
		}
	}

	var sinks []pwruntime.Sink
	if resolved.logs != nil {
		sinks = append(sinks, pwruntime.NewOtelSink(resolved.logs.Logger(loggerScope)))
	}
	// Exclusive routing is the rule: a record goes to the collector or to
	// stdout, not both. Development is the exception, because the terminal is
	// the surface the developer is actually watching and emptying it would make
	// the loop worse, not better.
	if resolved.logs == nil || env == EnvDevelopment {
		handler, err := stdoutHandler(config, minimum)
		if err != nil {
			return nil, err
		}
		sinks = append(sinks, pwruntime.NewSlogSink(handler))
	}
	resolved.backend = pwruntime.NewLogBackend(minimum, sinks...)
	setProcessBackend(resolved.backend)
	return resolved, nil
}

// loggerScope names the instrumentation scope of framework records.
const loggerScope = "github.com/shibukawa/popcornwave"

// processBackend is the emission policy of the running process.
//
// A package-level value exists because the startup summary and the query
// diagnostics warning are written before any request, so there is no context to
// carry a backend on. It is set once by buildMiddlewares and read afterwards.
var processBackend struct {
	sync.RWMutex
	backend *pwruntime.LogBackend
}

func setProcessBackend(backend *pwruntime.LogBackend) {
	processBackend.Lock()
	processBackend.backend = backend
	processBackend.Unlock()
}

// processLogger returns a logger for framework output that belongs to the
// process rather than to a request. Before configuration is parsed it falls
// back to the same stderr logger an unconfigured context gets.
func processLogger() Log {
	processBackend.RLock()
	backend := processBackend.backend
	processBackend.RUnlock()
	return pwruntime.NewLogger(context.Background(), backend)
}

// startOtel builds the exporter and both providers. Failure is returned rather
// than absorbed: an endpoint that was configured and cannot be used is a
// misconfiguration the operator has to see at startup.
func (resolved *observability) startOtel(config ObservabilityConfig, endpoint string) error {
	headers, err := otlpHeaders(config.Otel.Headers)
	if err != nil {
		return fmt.Errorf("observability.otel.headers: %w", err)
	}
	exporter, err := otlphttp.New(otlphttp.Config{
		Endpoint: endpoint,
		Headers:  headers,
		Timeout:  config.Otel.RequestTimeout,
	})
	if err != nil {
		return err
	}
	resource := resourceAttributes(config)
	resolved.traces = trace.NewProvider(
		trace.NewBatchProcessor(exporter, trace.BatchConfig{
			QueueSize:     config.Otel.QueueSize,
			MaxExportSize: config.Otel.MaxExportSize,
			FlushInterval: config.Otel.FlushInterval,
		}),
		trace.WithResourceAttributes(resource...),
	)
	resolved.logs = otellog.NewProvider(
		otellog.NewBatchProcessor(exporter, otellog.BatchConfig{
			QueueSize:     config.Otel.QueueSize,
			MaxExportSize: config.Otel.MaxExportSize,
			FlushInterval: config.Otel.FlushInterval,
		}),
		otellog.WithResourceAttributes(resource...),
	)
	resolved.tracing = true
	// The middleware and any handler calling pw.StartSpan resolve their tracer
	// through the default provider, so installing it is what makes spans real.
	trace.SetDefaultProvider(resolved.traces)
	otellog.SetDefaultProvider(resolved.logs)
	registerCleanup("otel", resolved.shutdown)
	return nil
}

// shutdown flushes both providers within the caller's deadline. Records still
// queued at that point are lost by design: shutdown is bounded.
func (resolved *observability) shutdown(ctx context.Context) error {
	if resolved == nil {
		return nil
	}
	var result error
	if resolved.traces != nil {
		result = errors.Join(result, resolved.traces.Shutdown(ctx))
	}
	if resolved.logs != nil {
		result = errors.Join(result, resolved.logs.Shutdown(ctx))
	}
	return result
}

// sinkCount reports how many destinations a record reaches.
func (resolved *observability) sinkCount() int { return resolved.backend.Sinks() }

// serviceNameOf returns the service.name of a resource attribute set.
func serviceNameOf(attributes []otel.Attribute) string {
	for _, attribute := range attributes {
		if attribute.Key == "service.name" {
			value, _ := attribute.Value.AsString()
			return value
		}
	}
	return ""
}

// otlpEndpoint resolves where telemetry is sent.
//
// The binding already maps OTEL_EXPORTER_OTLP_ENDPOINT onto the field, with the
// ordinary precedence, so a parsed configuration answers this on its own. The
// environment read remains for the path that never parses one: a test, or code
// that builds observability directly.
func otlpEndpoint(config ObservabilityConfig) string {
	if endpoint := strings.TrimSpace(config.Otel.Endpoint); endpoint != "" {
		return endpoint
	}
	return strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
}

// otlpHeaders parses the OTLP header list form, the same one the standard
// environment variable uses. Values are never logged.
func otlpHeaders(raw string) (http.Header, error) {
	headers := make(http.Header)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, value, found := strings.Cut(pair, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return nil, errors.New("each entry must be key=value")
		}
		headers.Add(key, strings.TrimSpace(value))
	}
	return headers, nil
}

// resourceAttributes identifies this service to the collector. The environment
// read is the same unparsed-path fallback otlpEndpoint keeps.
func resourceAttributes(config ObservabilityConfig) []otel.Attribute {
	service := strings.TrimSpace(config.ServiceName)
	if service == "" {
		service = strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	}
	if service == "" {
		service = executableName()
	}
	attributes := []otel.Attribute{otel.String("service.name", service)}
	for _, entry := range config.ResourceAttributes {
		key, value, found := strings.Cut(entry, "=")
		if key = strings.TrimSpace(key); found && key != "" {
			attributes = append(attributes, otel.String(key, strings.TrimSpace(value)))
		}
	}
	return attributes
}

// stdoutHandler builds the terminal or container-log encoder. The handler
// carries the same floor as the backend so a substituted handler cannot widen
// what the configuration allows.
func stdoutHandler(config ObservabilityConfig, minimum Level) (slog.Handler, error) {
	options := &slog.HandlerOptions{Level: slog.Level(minimum)}
	switch strings.ToLower(strings.TrimSpace(config.StdoutFormat)) {
	case "", StdoutFormatJSON:
		return slog.NewJSONHandler(os.Stdout, options), nil
	case StdoutFormatPlaintext:
		return slog.NewTextHandler(os.Stdout, options), nil
	default:
		return nil, fmt.Errorf("observability.stdout_format: must be %s or %s", StdoutFormatJSON, StdoutFormatPlaintext)
	}
}

// parseLevel maps a configured token to a severity. An empty value keeps the
// caller's default rather than silently becoming the lowest severity.
func parseLevel(value string, fallback Level) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return fallback, nil
	case "trace":
		return LevelTrace, nil
	case "debug":
		return LevelDebug, nil
	case "info":
		return LevelInfo, nil
	case "warn":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	case "off":
		return LevelOff, nil
	default:
		return 0, errors.New("must be trace, debug, info, warn, error, or off")
	}
}
