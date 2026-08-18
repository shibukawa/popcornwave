// Package log provides correlated and standalone OpenTelemetry log records.
package log

import (
	"context"
	"sync"
	"time"

	"github.com/shibukawa/popcornweb/contrib/otel"
	"github.com/shibukawa/popcornweb/contrib/otel/trace"
)

// SeverityNumber follows the OpenTelemetry log data model numeric ranges.
type SeverityNumber uint8

const (
	SeverityTrace SeverityNumber = 1
	SeverityDebug SeverityNumber = 5
	SeverityInfo  SeverityNumber = 9
	SeverityWarn  SeverityNumber = 13
	SeverityError SeverityNumber = 17
	SeverityFatal SeverityNumber = 21
)

// Record is emitted by application code. Timestamp defaults to time.Now.
type Record struct {
	Timestamp    time.Time
	ObservedTime time.Time
	Severity     SeverityNumber
	SeverityText string
	Body         string
	EventName    string
	Attributes   []otel.Attribute
}

// RecordData is the immutable, trace-correlated representation sent to a Processor.
type RecordData struct {
	Record
	TraceID            string
	SpanID             string
	TraceFlags         byte
	ScopeName          string
	ResourceAttributes []otel.Attribute
}

type Processor interface {
	OnEmit(RecordData)
	Shutdown(context.Context) error
}

type Provider struct {
	processor Processor
	resource  []otel.Attribute
}
type ProviderOption func(*Provider)

func WithResourceAttributes(attributes ...otel.Attribute) ProviderOption {
	copyOf := append([]otel.Attribute(nil), attributes...)
	return func(p *Provider) { p.resource = copyOf }
}
func NewProvider(processor Processor, options ...ProviderOption) *Provider {
	p := &Provider{processor: processor}
	for _, option := range options {
		option(p)
	}
	return p
}
func (p *Provider) Logger(name string) *Logger { return &Logger{provider: p, name: name} }
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.processor == nil {
		return nil
	}
	return p.processor.Shutdown(ctx)
}

var defaultState = struct {
	sync.RWMutex
	provider *Provider
}{provider: NewProvider(nil)}

func DefaultProvider() *Provider {
	defaultState.RLock()
	defer defaultState.RUnlock()
	return defaultState.provider
}
func SetDefaultProvider(provider *Provider) {
	if provider == nil {
		provider = NewProvider(nil)
	}
	defaultState.Lock()
	defaultState.provider = provider
	defaultState.Unlock()
}

type Logger struct {
	provider *Provider
	name     string
}

func (l *Logger) Emit(ctx context.Context, record Record) {
	if l == nil {
		return
	}
	provider := l.provider
	if provider == nil {
		provider = DefaultProvider()
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}
	if record.ObservedTime.IsZero() {
		record.ObservedTime = time.Now()
	}
	data := RecordData{Record: record, ScopeName: l.name, ResourceAttributes: provider.resource}
	data.Attributes = append([]otel.Attribute(nil), record.Attributes...)
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		data.TraceID, data.SpanID, data.TraceFlags = sc.TraceID(), sc.SpanID(), sc.TraceFlags()
	}
	if provider.processor != nil {
		provider.processor.OnEmit(data)
	}
}
