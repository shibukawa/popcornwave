// Package otlphttp exports traces and logs with OTLP/HTTP JSON.
package otlphttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/shibukawa/popcornwave/contrib/otel"
	otellog "github.com/shibukawa/popcornwave/contrib/otel/log"
	"github.com/shibukawa/popcornwave/contrib/otel/trace"
)

const maxResponseBody = 4096

type Config struct {
	Endpoint       string
	TracesEndpoint string
	LogsEndpoint   string
	Headers        http.Header
	Client         *http.Client
	Timeout        time.Duration
	// MaxRetries defaults to two. A negative value disables retries.
	MaxRetries int
}

type Exporter struct {
	tracesURL string
	logsURL   string
	headers   http.Header
	client    *http.Client
	timeout   time.Duration
	retries   int
}

// New creates an exporter. Endpoint is a base URL; signal-specific endpoints
// are complete URLs. Static headers are defensively copied.
func New(config Config) (*Exporter, error) {
	if config.Endpoint == "" {
		config.Endpoint = "https://localhost:4318"
	}
	if config.TracesEndpoint == "" {
		config.TracesEndpoint = strings.TrimRight(config.Endpoint, "/") + "/v1/traces"
	}
	if config.LogsEndpoint == "" {
		config.LogsEndpoint = strings.TrimRight(config.Endpoint, "/") + "/v1/logs"
	}
	if err := validEndpoint(config.TracesEndpoint); err != nil {
		return nil, fmt.Errorf("otel otlphttp traces endpoint: %w", err)
	}
	if err := validEndpoint(config.LogsEndpoint); err != nil {
		return nil, fmt.Errorf("otel otlphttp logs endpoint: %w", err)
	}
	if config.Client == nil {
		config.Client = http.DefaultClient
	}
	config.Client = untraced(config.Client)
	if config.Headers == nil {
		config.Headers = make(http.Header)
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 2
	} else if config.MaxRetries < 0 {
		config.MaxRetries = 0
	}
	return &Exporter{tracesURL: config.TracesEndpoint, logsURL: config.LogsEndpoint, headers: config.Headers.Clone(), client: config.Client, timeout: config.Timeout, retries: config.MaxRetries}, nil
}

// NewFromEnv reads the minimal standard OTLP environment configuration.
func NewFromEnv() (*Exporter, error) {
	timeout := 10 * time.Second
	if raw := os.Getenv("OTEL_EXPORTER_OTLP_TIMEOUT"); raw != "" {
		milliseconds, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || milliseconds <= 0 {
			return nil, fmt.Errorf("otel otlphttp: invalid OTEL_EXPORTER_OTLP_TIMEOUT %q", raw)
		}
		timeout = time.Duration(milliseconds) * time.Millisecond
	}
	headers, err := parseHeaders(os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"))
	if err != nil {
		return nil, err
	}
	return New(Config{
		Endpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"), TracesEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"),
		LogsEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"), Headers: headers, Timeout: timeout, MaxRetries: 2,
	})
}

// ResourceFromEnv returns the minimal standard service resource configuration.
func ResourceFromEnv() []otel.Attribute {
	if service := os.Getenv("OTEL_SERVICE_NAME"); service != "" {
		return []otel.Attribute{otel.String("service.name", service)}
	}
	return nil
}

func (e *Exporter) ExportSpans(ctx context.Context, spans []trace.SpanData) error {
	if len(spans) == 0 {
		return nil
	}
	resources := make([]resourceSpans, 0, 1)
	for _, span := range spans {
		encoded := otlpSpan{
			TraceID: span.SpanContext.TraceID(), SpanID: span.SpanContext.SpanID(), ParentSpanID: span.ParentSpanID,
			TraceState: span.SpanContext.TraceState(), Flags: uint32(span.SpanContext.TraceFlags()), Name: span.Name,
			Kind: uint32(span.Kind), StartTimeUnixNano: unixNano(span.StartTime), EndTimeUnixNano: unixNano(span.EndTime),
			Attributes: attributes(span.Attributes), Status: otlpStatus{Code: uint32(span.Status), Message: span.StatusDescription},
		}
		for _, event := range span.Events {
			encoded.Events = append(encoded.Events, otlpEvent{Name: event.Name, TimeUnixNano: unixNano(event.Time), Attributes: attributes(event.Attributes)})
		}
		if len(resources) == 0 || resources[len(resources)-1].ScopeSpans[0].Scope.Name != span.ScopeName ||
			!slices.Equal(resources[len(resources)-1].sourceAttributes, span.ResourceAttributes) {
			resources = append(resources, resourceSpans{
				Resource:         resource{Attributes: attributes(span.ResourceAttributes)},
				ScopeSpans:       []scopeSpans{{Scope: scope{Name: span.ScopeName}}},
				sourceAttributes: span.ResourceAttributes,
			})
		}
		group := &resources[len(resources)-1].ScopeSpans[0]
		group.Spans = append(group.Spans, encoded)
	}
	return e.send(ctx, e.tracesURL, traceRequest{ResourceSpans: resources})
}

func (e *Exporter) ExportLogs(ctx context.Context, records []otellog.RecordData) error {
	if len(records) == 0 {
		return nil
	}
	resources := make([]resourceLogs, 0, 1)
	for _, record := range records {
		encoded := logRecord{
			TimeUnixNano: unixNano(record.Timestamp), ObservedTimeUnixNano: unixNano(record.ObservedTime),
			SeverityNumber: uint32(record.Severity), SeverityText: record.SeverityText, Body: anyValue{StringValue: &record.Body},
			Attributes: attributes(record.Attributes), TraceID: record.TraceID, SpanID: record.SpanID,
			Flags: uint32(record.TraceFlags), EventName: record.EventName,
		}
		if len(resources) == 0 || resources[len(resources)-1].ScopeLogs[0].Scope.Name != record.ScopeName ||
			!slices.Equal(resources[len(resources)-1].sourceAttributes, record.ResourceAttributes) {
			resources = append(resources, resourceLogs{
				Resource:         resource{Attributes: attributes(record.ResourceAttributes)},
				ScopeLogs:        []scopeLogs{{Scope: scope{Name: record.ScopeName}}},
				sourceAttributes: record.ResourceAttributes,
			})
		}
		group := &resources[len(resources)-1].ScopeLogs[0]
		group.LogRecords = append(group.LogRecords, encoded)
	}
	return e.send(ctx, e.logsURL, logRequest{ResourceLogs: resources})
}

func (e *Exporter) send(ctx context.Context, endpoint string, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("otel otlphttp: encode: %w", err)
	}
	requestCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	var last error
	for attempt := 0; attempt <= e.retries; attempt++ {
		if attempt > 0 {
			base := time.Duration(100*(1<<(attempt-1))) * time.Millisecond
			delay := base + time.Duration(time.Now().UnixNano()%int64(base/2+1))
			select {
			case <-time.After(delay):
			case <-requestCtx.Done():
				return errors.Join(last, requestCtx.Err())
			}
		}
		req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header = e.headers.Clone()
		req.Header.Set("Content-Type", "application/json")
		response, err := e.client.Do(req)
		if err != nil {
			last = fmt.Errorf("otel otlphttp: send: %w", err)
			continue
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBody+1))
		closeErr := response.Body.Close()
		if readErr != nil {
			return fmt.Errorf("otel otlphttp: read response: %w", readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("otel otlphttp: close response: %w", closeErr)
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return nil
		}
		if len(responseBody) > maxResponseBody {
			responseBody = responseBody[:maxResponseBody]
		}
		last = fmt.Errorf("otel otlphttp: status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
		if !transient(response.StatusCode) {
			return last
		}
	}
	return last
}

func validEndpoint(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("endpoint must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("endpoint has no host")
	}
	return nil
}
func transient(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}
func parseHeaders(raw string) (http.Header, error) {
	headers := make(http.Header)
	if raw == "" {
		return headers, nil
	}
	for _, item := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(item, "=")
		key = strings.TrimSpace(key)
		if !ok || !validHeaderKey(key) {
			return nil, errors.New("otel otlphttp: invalid OTEL_EXPORTER_OTLP_HEADERS")
		}
		decoded, err := url.QueryUnescape(strings.TrimSpace(value))
		if err != nil || strings.ContainsAny(decoded, "\r\n") {
			return nil, errors.New("otel otlphttp: invalid OTEL_EXPORTER_OTLP_HEADERS")
		}
		headers.Add(key, decoded)
	}
	return headers, nil
}

func validHeaderKey(key string) bool {
	if key == "" {
		return false
	}
	for i := 0; i < len(key); i++ {
		b := key[i]
		if !((b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", rune(b))) {
			return false
		}
	}
	return true
}

func unixNano(value time.Time) string {
	if value.IsZero() {
		return "0"
	}
	return strconv.FormatInt(value.UnixNano(), 10)
}

type anyValue struct {
	StringValue *string  `json:"stringValue,omitempty"`
	BoolValue   *bool    `json:"boolValue,omitempty"`
	IntValue    *string  `json:"intValue,omitempty"`
	DoubleValue *float64 `json:"doubleValue,omitempty"`
}
type keyValue struct {
	Key   string   `json:"key"`
	Value anyValue `json:"value"`
}

func attributes(input []otel.Attribute) []keyValue {
	if len(input) == 0 {
		return nil
	}
	result := make([]keyValue, 0, len(input))
	for _, attribute := range input {
		if attribute.Key == "" {
			continue
		}
		value := anyValue{}
		switch attribute.Value.Kind() {
		case otel.StringKind:
			v, _ := attribute.Value.AsString()
			value.StringValue = &v
		case otel.BoolKind:
			v, _ := attribute.Value.AsBool()
			value.BoolValue = &v
		case otel.Int64Kind:
			v, _ := attribute.Value.AsInt64()
			encoded := strconv.FormatInt(v, 10)
			value.IntValue = &encoded
		case otel.Float64Kind:
			v, _ := attribute.Value.AsFloat64()
			value.DoubleValue = &v
		default:
			continue
		}
		result = append(result, keyValue{Key: attribute.Key, Value: value})
	}
	return result
}

type resource struct {
	Attributes []keyValue `json:"attributes,omitempty"`
}
type scope struct {
	Name string `json:"name,omitempty"`
}
type otlpStatus struct {
	Message string `json:"message,omitempty"`
	Code    uint32 `json:"code,omitempty"`
}
type otlpEvent struct {
	TimeUnixNano string     `json:"timeUnixNano"`
	Name         string     `json:"name"`
	Attributes   []keyValue `json:"attributes,omitempty"`
}
type otlpSpan struct {
	TraceID           string      `json:"traceId"`
	SpanID            string      `json:"spanId"`
	TraceState        string      `json:"traceState,omitempty"`
	ParentSpanID      string      `json:"parentSpanId,omitempty"`
	Flags             uint32      `json:"flags,omitempty"`
	Name              string      `json:"name"`
	Kind              uint32      `json:"kind,omitempty"`
	StartTimeUnixNano string      `json:"startTimeUnixNano"`
	EndTimeUnixNano   string      `json:"endTimeUnixNano"`
	Attributes        []keyValue  `json:"attributes,omitempty"`
	Events            []otlpEvent `json:"events,omitempty"`
	Status            otlpStatus  `json:"status,omitempty"`
}
type scopeSpans struct {
	Scope scope      `json:"scope"`
	Spans []otlpSpan `json:"spans"`
}
type resourceSpans struct {
	Resource         resource         `json:"resource"`
	ScopeSpans       []scopeSpans     `json:"scopeSpans"`
	sourceAttributes []otel.Attribute `json:"-"`
}
type traceRequest struct {
	ResourceSpans []resourceSpans `json:"resourceSpans"`
}

type logRecord struct {
	TimeUnixNano         string     `json:"timeUnixNano"`
	ObservedTimeUnixNano string     `json:"observedTimeUnixNano,omitempty"`
	SeverityNumber       uint32     `json:"severityNumber,omitempty"`
	SeverityText         string     `json:"severityText,omitempty"`
	Body                 anyValue   `json:"body"`
	Attributes           []keyValue `json:"attributes,omitempty"`
	Flags                uint32     `json:"flags,omitempty"`
	TraceID              string     `json:"traceId,omitempty"`
	SpanID               string     `json:"spanId,omitempty"`
	EventName            string     `json:"eventName,omitempty"`
}
type scopeLogs struct {
	Scope      scope       `json:"scope"`
	LogRecords []logRecord `json:"logRecords"`
}
type resourceLogs struct {
	Resource         resource         `json:"resource"`
	ScopeLogs        []scopeLogs      `json:"scopeLogs"`
	sourceAttributes []otel.Attribute `json:"-"`
}
type logRequest struct {
	ResourceLogs []resourceLogs `json:"resourceLogs"`
}
