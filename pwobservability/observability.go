package pwobservability

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
	"github.com/shibukawa/popcornwave/pwconfig"
	"github.com/shibukawa/popcornwave/pwruntime"
)

// Stdout record formats.
const (
	StdoutFormatJSON      = "json"
	StdoutFormatPlaintext = "plaintext"
)

// Resolved is the resolved emission and tracing state of one process.
type Resolved struct {
	backend *pwruntime.LogBackend
	traces  *trace.Provider
	logs    *otellog.Provider
	// tracing reports whether a request root span should be created. It is
	// false when nothing exports, because an unexported span is pure cost.
	tracing  bool
	cleanups []Cleanup
}

// Cleanup is one release this process owes, named so a runtime keeps one per
// name across repeated initialization.
type Cleanup struct {
	Name  string
	Close func(context.Context) error
}

// Build resolves configuration and the standard OTLP environment
// into one emission policy.
//
// The environment matters as much as the file here: api:cli-dev injects
// OTEL_EXPORTER_OTLP_ENDPOINT into the process it starts, so a developer gets
// traces and correlated logs without writing configuration at all.
func Build(config pwconfig.ObservabilityConfig, env string) (*Resolved, error) {
	minimum, err := ParseLevel(config.MinimumLevel, pwruntime.LevelInfo)
	if err != nil {
		return nil, fmt.Errorf("observability.minimum_level: %w", err)
	}
	resolved := &Resolved{}
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
	if resolved.logs == nil || env == pwconfig.EnvDevelopment {
		handler, err := stdoutHandler(config, minimum, env)
		if err != nil {
			return nil, err
		}
		sinks = append(sinks, pwruntime.NewSlogSink(handler))
	}
	fileSink, fileCloser := developmentSink(config, minimum, env, os.Stderr)
	if fileSink != nil {
		sinks = append(sinks, fileSink)
		resolved.cleanups = append(resolved.cleanups, Cleanup{
			Name: "development log", Close: func(context.Context) error { return fileCloser.Close() }})
	}
	resolved.backend = pwruntime.NewLogBackend(minimum, sinks...)
	SetProcessBackend(resolved.backend)
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

// SwapProcessBackend installs a backend and returns the restore, for a test
// that has to read what the framework says about itself at startup. Startup
// records are written before any request, so there is no context to capture one
// through and the process-wide backend is the only seam there is.
func SwapProcessBackend(backend *pwruntime.LogBackend) func() {
	processBackend.RLock()
	previous := processBackend.backend
	processBackend.RUnlock()
	SetProcessBackend(backend)
	return func() { SetProcessBackend(previous) }
}

func SetProcessBackend(backend *pwruntime.LogBackend) {
	processBackend.Lock()
	processBackend.backend = backend
	processBackend.Unlock()
}

// ProcessLogger returns a logger for framework output that belongs to the
// process rather than to a request. Before configuration is parsed it falls
// back to the same stderr logger an unconfigured context gets.
func ProcessLogger() pwruntime.Logger {
	processBackend.RLock()
	backend := processBackend.backend
	processBackend.RUnlock()
	return pwruntime.NewLogger(context.Background(), backend)
}

// startOtel builds the exporter and both providers. Failure is returned rather
// than absorbed: an endpoint that was configured and cannot be used is a
// misconfiguration the operator has to see at startup.
func (resolved *Resolved) startOtel(config pwconfig.ObservabilityConfig, endpoint string) error {
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
	resolved.cleanups = append(resolved.cleanups, Cleanup{Name: "otel", Close: resolved.Shutdown})
	return nil
}

// Shutdown flushes both providers within the caller's deadline. Records still
// queued at that point are lost by design: shutdown is bounded.
func (resolved *Resolved) Shutdown(ctx context.Context) error {
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

// SinkCount reports how many destinations a record reaches.
func (resolved *Resolved) SinkCount() int { return resolved.backend.Sinks() }

// Backend is the emission policy every request logger is built from.
func (resolved *Resolved) Backend() *pwruntime.LogBackend { return resolved.backend }

// Tracing reports whether a request root span should be created.
func (resolved *Resolved) Tracing() bool { return resolved.tracing }

// Cleanups are what this process opened and must release, in the order they
// were opened. They are returned rather than registered because the shutdown
// order is the runtime's: a log sink has to outlive whatever still logs.
func (resolved *Resolved) Cleanups() []Cleanup { return resolved.cleanups }

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
func otlpEndpoint(config pwconfig.ObservabilityConfig) string {
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
func resourceAttributes(config pwconfig.ObservabilityConfig) []otel.Attribute {
	service := strings.TrimSpace(config.ServiceName)
	if service == "" {
		service = strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	}
	if service == "" {
		service = pwconfig.ExecutableName()
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
func stdoutHandler(config pwconfig.ObservabilityConfig, minimum pwruntime.Level, env string) (slog.Handler, error) {
	options := &slog.HandlerOptions{Level: slog.Level(minimum)}
	switch strings.ToLower(strings.TrimSpace(config.StdoutFormat)) {
	case "":
		// Unset resolves by environment rather than to one format everywhere.
		// The development stream is read by a person in a terminal, where the
		// slog text encoding is legible and a JSON object per line is not.
		// Every other environment feeds a collector, which wants the opposite.
		if env == pwconfig.EnvDevelopment {
			return slog.NewTextHandler(os.Stdout, options), nil
		}
		return slog.NewJSONHandler(os.Stdout, options), nil
	case StdoutFormatJSON:
		return slog.NewJSONHandler(os.Stdout, options), nil
	case StdoutFormatPlaintext:
		return slog.NewTextHandler(os.Stdout, options), nil
	default:
		return nil, fmt.Errorf("observability.stdout_format: must be %s or %s", StdoutFormatJSON, StdoutFormatPlaintext)
	}
}

// ParseLevel maps a configured token to a severity. An empty value keeps the
// caller's default rather than silently becoming the lowest severity.
func ParseLevel(value string, fallback pwruntime.Level) (pwruntime.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return fallback, nil
	case "trace":
		return pwruntime.LevelTrace, nil
	case "debug":
		return pwruntime.LevelDebug, nil
	case "info":
		return pwruntime.LevelInfo, nil
	case "warn":
		return pwruntime.LevelWarn, nil
	case "error":
		return pwruntime.LevelError, nil
	case "off":
		return pwruntime.LevelOff, nil
	default:
		return 0, errors.New("must be trace, debug, info, warn, error, or off")
	}
}
