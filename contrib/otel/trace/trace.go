// Package trace provides a small, explicit OpenTelemetry tracing subset.
//
// A Sampler decides at the root whether a trace is recorded, so a process
// exporting straight to a collection backend can decline a span before it costs
// anything. Tail sampling still belongs to a collector: only something holding
// the finished trace can keep it for what happened in it.
package trace

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/contrib/otel"
)

// SpanKind follows the OTLP SpanKind numeric values.
type SpanKind uint8

const (
	SpanKindInternal SpanKind = iota + 1
	SpanKindServer
	SpanKindClient
	SpanKindProducer
	SpanKindConsumer
)

// StatusCode follows the OTLP StatusCode numeric values.
type StatusCode uint8

const (
	StatusUnset StatusCode = iota
	StatusOK
	StatusError
)

// SpanContext is the W3C Trace Context carried between processes.
type SpanContext struct {
	traceID    [16]byte
	spanID     [8]byte
	traceFlags byte
	traceState string
	remote     bool
}

// NewSpanContext validates and constructs a span context from lowercase or uppercase hex IDs.
func NewSpanContext(traceID, spanID string, traceFlags byte, traceState string, remote bool) (SpanContext, error) {
	var sc SpanContext
	if len(traceID) != 32 || len(spanID) != 16 {
		return sc, errors.New("otel trace: invalid trace or span ID length")
	}
	if _, err := hex.Decode(sc.traceID[:], []byte(traceID)); err != nil || allZero(sc.traceID[:]) {
		return SpanContext{}, errors.New("otel trace: invalid trace ID")
	}
	if _, err := hex.Decode(sc.spanID[:], []byte(spanID)); err != nil || allZero(sc.spanID[:]) {
		return SpanContext{}, errors.New("otel trace: invalid span ID")
	}
	sc.traceFlags = traceFlags
	sc.traceState = traceState
	sc.remote = remote
	return sc, nil
}

func (s SpanContext) IsValid() bool { return !allZero(s.traceID[:]) && !allZero(s.spanID[:]) }
func (s SpanContext) TraceID() string {
	if allZero(s.traceID[:]) {
		return ""
	}
	return hex.EncodeToString(s.traceID[:])
}
func (s SpanContext) SpanID() string {
	if allZero(s.spanID[:]) {
		return ""
	}
	return hex.EncodeToString(s.spanID[:])
}
func (s SpanContext) TraceFlags() byte   { return s.traceFlags }
func (s SpanContext) TraceState() string { return s.traceState }
func (s SpanContext) IsRemote() bool     { return s.remote }

type contextKey struct{}
type contextValue struct {
	spanContext SpanContext
	tracer      *Tracer
	span        *Span
}

// ContextWithSpanContext installs a propagation-only span context.
func ContextWithSpanContext(ctx context.Context, sc SpanContext) context.Context {
	if !sc.IsValid() {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, contextValue{spanContext: sc})
}

// SpanContextFromContext returns the active or extracted span context.
func SpanContextFromContext(ctx context.Context) SpanContext {
	if ctx == nil {
		return SpanContext{}
	}
	v, _ := ctx.Value(contextKey{}).(contextValue)
	return v.spanContext
}

// SpanFromContext returns the span Start installed on ctx, or nil for a
// context carrying no span or only an extracted remote span context. One
// lookup reaches the innermost span; its ancestors are reached through
// Parent without touching the context again.
func SpanFromContext(ctx context.Context) *Span {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(contextKey{}).(contextValue)
	return v.span
}

// Event is a timestamped span event.
type Event struct {
	Name       string
	Time       time.Time
	Attributes []otel.Attribute
}

// SpanData is the immutable representation delivered to a Processor.
type SpanData struct {
	Name               string
	SpanContext        SpanContext
	ParentSpanID       string
	Kind               SpanKind
	StartTime          time.Time
	EndTime            time.Time
	Attributes         []otel.Attribute
	Events             []Event
	Status             StatusCode
	StatusDescription  string
	ScopeName          string
	ResourceAttributes []otel.Attribute
}

// Processor receives completed spans and owns exporter shutdown.
type Processor interface {
	OnEnd(SpanData)
	Shutdown(context.Context) error
}

// Provider owns tracers, their processor, and the sampler every tracer under it
// consults.
type Provider struct {
	processor Processor
	resource  []otel.Attribute
	sampler   Sampler
}

type ProviderOption func(*Provider)

// WithResourceAttributes adds resource attributes copied into every completed span.
func WithResourceAttributes(attributes ...otel.Attribute) ProviderOption {
	copyOf := append([]otel.Attribute(nil), attributes...)
	return func(p *Provider) { p.resource = copyOf }
}

// WithSampler selects which traces are recorded. The default is
// ParentBased(AlwaysOn), which records everything a caller has not already
// declined.
func WithSampler(sampler Sampler) ProviderOption {
	return func(p *Provider) {
		if sampler != nil {
			p.sampler = sampler
		}
	}
}

func NewProvider(processor Processor, options ...ProviderOption) *Provider {
	p := &Provider{processor: processor, sampler: ParentBased(AlwaysOn())}
	for _, option := range options {
		option(p)
	}
	return p
}

// Sampler reports the sampler in force, for a diagnostic that has to say what
// this process will keep.
func (p *Provider) Sampler() Sampler {
	if p == nil || p.sampler == nil {
		return ParentBased(AlwaysOn())
	}
	return p.sampler
}

func (p *Provider) Tracer(name string) *Tracer {
	if p == nil {
		p = DefaultProvider()
	}
	return &Tracer{provider: p, name: name}
}

func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.processor == nil {
		return nil
	}
	return p.processor.Shutdown(ctx)
}

var (
	defaultMu       sync.RWMutex
	defaultProvider = NewProvider(nil)
)

func DefaultProvider() *Provider {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultProvider
}
func SetDefaultProvider(provider *Provider) {
	if provider == nil {
		provider = NewProvider(nil)
	}
	defaultMu.Lock()
	defaultProvider = provider
	defaultMu.Unlock()
}

// Tracer creates spans for one instrumentation scope.
type Tracer struct {
	provider *Provider
	name     string
}

type startConfig struct {
	kind       SpanKind
	attributes []otel.Attribute
	start      time.Time
}
type StartOption func(*startConfig)

func WithSpanKind(kind SpanKind) StartOption { return func(c *startConfig) { c.kind = kind } }
func WithAttributes(attributes ...otel.Attribute) StartOption {
	copyOf := append([]otel.Attribute(nil), attributes...)
	return func(c *startConfig) { c.attributes = append(c.attributes, copyOf...) }
}
func WithStartTime(t time.Time) StartOption { return func(c *startConfig) { c.start = t } }

// Start creates a child of the span context in ctx, or a new root span.
func (t *Tracer) Start(ctx context.Context, name string, options ...StartOption) (context.Context, *Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg := startConfig{kind: SpanKindInternal, start: time.Now()}
	for _, option := range options {
		option(&cfg)
	}
	parentValue, _ := ctx.Value(contextKey{}).(contextValue)
	parent := parentValue.spanContext
	sc := SpanContext{}
	if parent.IsValid() {
		sc.traceID = parent.traceID
		sc.traceState = parent.traceState
	} else {
		fillID(sc.traceID[:])
	}
	fillID(sc.spanID[:])
	if allZero(sc.traceID[:]) {
		sc.traceID[15] = 1
	}
	if allZero(sc.spanID[:]) {
		sc.spanID[7] = 1
	}
	// Every trace flag but the sampled bit belongs to the parent; that one bit is
	// this decision's, and a root has no parent flags to carry.
	sc.traceFlags = parent.traceFlags &^ flagSampled
	recording := t.provider.Sampler().ShouldSample(SamplingParameters{
		Parent: parent, TraceID: sc.traceID, Name: name, Kind: cfg.kind,
	})
	if recording {
		sc.traceFlags |= flagSampled
	}
	span := &Span{tracer: t, parent: parentValue.span, recording: recording}
	if recording {
		// cfg.attributes is built fresh for this call (WithAttributes copies its
		// input), so the span owns it without another copy.
		span.data = SpanData{
			Name: name, SpanContext: sc, Kind: cfg.kind, StartTime: cfg.start,
			Attributes: cfg.attributes, ScopeName: t.name,
			ResourceAttributes: t.provider.resource,
		}
		if parent.IsValid() {
			span.data.ParentSpanID = parent.SpanID()
		}
	} else {
		// An unsampled span keeps only what is read outside the exporter: the
		// span context, for the traceparent it injects and the trace_id a log
		// record correlates on. The name, the attributes, the start time, and
		// the resource are all dropped here rather than at export, so they cost
		// nothing to hold. The value is still per call because Parent and Root
		// are pointer chases that requirement:context-lookup-performance keeps
		// free, and a shared instance would have no parent to point at.
		span.data = SpanData{SpanContext: sc}
	}
	ctx = context.WithValue(ctx, contextKey{}, contextValue{spanContext: sc, tracer: t, span: span})
	return ctx, span
}

// Start creates a child span with the active tracer, falling back to the default provider.
func Start(ctx context.Context, name string, options ...StartOption) (context.Context, *Span) {
	if ctx != nil {
		if v, ok := ctx.Value(contextKey{}).(contextValue); ok && v.tracer != nil {
			return v.tracer.Start(ctx, name, options...)
		}
	}
	return DefaultProvider().Tracer("github.com/shibukawa/popcornwave/contrib/otel/trace").Start(ctx, name, options...)
}

// Span is safe for concurrent attribute/event updates and idempotent End calls.
type Span struct {
	mu     sync.Mutex
	tracer *Tracer
	// parent is the local parent span, fixed at Start. It is nil for a root
	// span and for a child of an extracted remote span context, which has no
	// local Span value to point at.
	parent *Span
	data   SpanData
	ended  bool
	// recording is the sampler's decision, fixed at Start. A false value makes
	// every mutation a no-op and keeps End from reaching the processor, so an
	// unsampled span never occupies a queue slot.
	recording bool
}

// IsRecording reports whether this span will be exported. A caller that would
// have to compute a value only to attach it can skip the work when this is
// false; a caller that already has the value need not ask.
func (s *Span) IsRecording() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recording && !s.ended
}

// Parent returns the local parent span, or nil for a root or a span whose
// parent is remote. The chain is fixed at Start, so walking ancestors is a
// pointer chase with no context traversal.
func (s *Span) Parent() *Span {
	if s == nil {
		return nil
	}
	return s.parent
}

// Root follows Parent to the outermost local span, which for a request served
// by pw is the request root span.
func (s *Span) Root() *Span {
	if s == nil {
		return nil
	}
	for s.parent != nil {
		s = s.parent
	}
	return s
}

func (s *Span) SpanContext() SpanContext { s.mu.Lock(); defer s.mu.Unlock(); return s.data.SpanContext }
func (s *Span) SetAttributes(attributes ...otel.Attribute) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recording && !s.ended {
		s.data.Attributes = append(s.data.Attributes, attributes...)
	}
}
func (s *Span) AddEvent(name string, attributes ...otel.Attribute) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recording && !s.ended {
		s.data.Events = append(s.data.Events, Event{Name: name, Time: time.Now(), Attributes: append([]otel.Attribute(nil), attributes...)})
	}
}
func (s *Span) RecordError(err error) {
	if err == nil {
		return
	}
	s.AddEvent("exception", otel.String("exception.type", fmt.Sprintf("%T", err)), otel.String("exception.message", err.Error()))
}
func (s *Span) SetStatus(code StatusCode, description string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.recording && !s.ended {
		s.data.Status, s.data.StatusDescription = code, description
	}
}
func (s *Span) End() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return
	}
	s.ended = true
	if !s.recording {
		// The drop is here rather than in the exporter, so an unsampled trace
		// never takes a queue slot and never moves the dropped-record count that
		// a full queue is supposed to mean.
		s.mu.Unlock()
		return
	}
	s.data.EndTime = time.Now()
	// ended is set, so SetAttributes and AddEvent can no longer append: the
	// slices leave the lock without a defensive copy.
	data := s.data
	s.mu.Unlock()
	if s.tracer != nil && s.tracer.provider.processor != nil {
		s.tracer.provider.processor.OnEnd(data)
	}
}

func allZero(value []byte) bool {
	for _, b := range value {
		if b != 0 {
			return false
		}
	}
	return true
}

var fallbackID = struct {
	sync.Mutex
	counter uint64
}{}

func fillID(destination []byte) {
	if _, err := rand.Read(destination); err == nil && !allZero(destination) {
		return
	}
	// Trace IDs are correlation identifiers, not secrets. This time/counter
	// fallback keeps Start usable on targets where the system RNG is absent.
	fallbackID.Lock()
	fallbackID.counter++
	counter := fallbackID.counter
	fallbackID.Unlock()
	stamp := uint64(time.Now().UnixNano())
	for offset := 0; offset < len(destination); offset += 8 {
		var block [8]byte
		binary.BigEndian.PutUint64(block[:], stamp^counter^uint64(offset))
		copy(destination[offset:], block[:])
	}
	if allZero(destination) {
		destination[len(destination)-1] = 1
	}
}

// ValueStore is a request value that carries its own state instead of being
// replaced by a derived copy, which is how a transport without derivable
// contexts publishes request state.
type ValueStore interface {
	SetUserValue(key, value any)
}

// StoreContext copies the active span of ctx onto a request value.
//
// It exists because a span reaches its readers through the context, and a
// transport whose request value is the context cannot be handed a derived one.
// Copying the value rather than exporting the key keeps the key unexported: an
// exported span key is one any code can write, and a written span is one the
// exporter believes.
func StoreContext(store ValueStore, ctx context.Context) {
	if store == nil || ctx == nil {
		return
	}
	if value, ok := ctx.Value(contextKey{}).(contextValue); ok {
		store.SetUserValue(contextKey{}, value)
	}
}
