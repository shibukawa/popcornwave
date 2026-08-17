// Package metric provides a small, explicit OpenTelemetry metrics subset.
//
// It aggregates in the process and exports on an interval, which is what makes
// it the signal that survives sampling: a trace records one request and a
// sampler may decline it, while an instrument counts every request and costs one
// export per interval whatever the traffic is.
//
// The set is deliberately narrow. There are four synchronous instruments, one
// observable form for a value that is read rather than recorded, and no views,
// no exemplars, and no per-instrument configuration beyond the bucket bounds a
// histogram is created with.
package metric

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shibukawa/popcornwave/contrib/otel"
)

// Temporality selects what an exported point covers.
type Temporality uint8

const (
	// DeltaTemporality reports what happened since the previous export. It
	// suits a short-lived instance and a reader that charts a session without
	// differencing, and it loses the counts of any export that fails.
	DeltaTemporality Temporality = iota
	// CumulativeTemporality reports the total since process start, so a failed
	// export is repaired by the next one at the cost of a reader that has to
	// notice when a series restarted.
	CumulativeTemporality
)

// InstrumentKind names how a point is read, which is what the OTLP encoding
// needs to choose a message.
type InstrumentKind uint8

const (
	KindCounter InstrumentKind = iota + 1
	KindUpDownCounter
	KindHistogram
	// KindGauge is an observable's kind: a value that is sampled at collection
	// rather than accumulated between collections.
	KindGauge
)

// DurationBounds are the semantic-convention histogram boundaries for a
// duration in seconds. They are the defaults every backend's dashboards were
// built against, which is worth more than boundaries fitted to this framework.
var DurationBounds = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

// SizeBounds are byte-oriented boundaries, from a few hundred bytes to a few
// megabytes. The specification names none for a size, so these are this
// framework's, chosen to straddle the range an HTML response occupies.
var SizeBounds = []float64{256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304}

// NumberPoint is one exported value for one attribute set.
type NumberPoint struct {
	Attributes []otel.Attribute
	Value      float64
	Start      time.Time
	Time       time.Time
}

// HistogramPoint is one exported distribution for one attribute set.
type HistogramPoint struct {
	Attributes   []otel.Attribute
	Count        uint64
	Sum          float64
	Min          float64
	Max          float64
	Bounds       []float64
	BucketCounts []uint64
	Start        time.Time
	Time         time.Time
}

// Data is one instrument's collected points.
type Data struct {
	Name        string
	Unit        string
	Description string
	Kind        InstrumentKind
	Temporality Temporality
	Monotonic   bool
	Numbers     []NumberPoint
	Histograms  []HistogramPoint
}

// ScopeData groups one instrumentation scope's instruments.
type ScopeData struct {
	Scope   string
	Metrics []Data
}

// ResourceData is one collection of one process.
type ResourceData struct {
	ResourceAttributes []otel.Attribute
	Scopes             []ScopeData
}

// Exporter sends one collection to a telemetry backend.
type Exporter interface {
	ExportMetrics(context.Context, ResourceData) error
}

// Provider owns meters and the aggregation every instrument under it writes to.
type Provider struct {
	mu          sync.RWMutex
	resource    []otel.Attribute
	temporality Temporality
	scopes      []*scope
	byName      map[string]*scope
	start       time.Time
}

type ProviderOption func(*Provider)

// WithResourceAttributes identifies the process in every collection.
func WithResourceAttributes(attributes ...otel.Attribute) ProviderOption {
	copyOf := append([]otel.Attribute(nil), attributes...)
	return func(p *Provider) { p.resource = copyOf }
}

// WithTemporality selects delta or cumulative export.
func WithTemporality(temporality Temporality) ProviderOption {
	return func(p *Provider) { p.temporality = temporality }
}

func NewProvider(options ...ProviderOption) *Provider {
	p := &Provider{byName: map[string]*scope{}, start: time.Now()}
	for _, option := range options {
		option(p)
	}
	return p
}

// Meter returns the instrument factory for one instrumentation scope. Repeated
// calls with one name share instruments, so a package may take its meter
// wherever it is convenient rather than threading one value through.
func (p *Provider) Meter(name string) *Meter {
	if p == nil {
		return &Meter{}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	existing, ok := p.byName[name]
	if !ok {
		existing = &scope{provider: p, name: name, byName: map[string]*instrument{}}
		p.byName[name] = existing
		p.scopes = append(p.scopes, existing)
	}
	return &Meter{scope: existing}
}

// Collect drains every instrument into one exportable value.
//
// Under delta temporality the accumulators are reset here, which is what makes
// the next collection cover only the interval after this one.
func (p *Provider) Collect() ResourceData {
	if p == nil {
		return ResourceData{}
	}
	now := time.Now()
	p.mu.RLock()
	scopes := append([]*scope(nil), p.scopes...)
	temporality, resource, start := p.temporality, p.resource, p.start
	p.mu.RUnlock()
	collected := ResourceData{ResourceAttributes: resource}
	for _, current := range scopes {
		metrics := current.collect(temporality, start, now)
		if len(metrics) == 0 {
			continue
		}
		collected.Scopes = append(collected.Scopes, ScopeData{Scope: current.name, Metrics: metrics})
	}
	if temporality == DeltaTemporality {
		p.mu.Lock()
		p.start = now
		p.mu.Unlock()
	}
	return collected
}

// scope holds one instrumentation scope's instruments in creation order, so a
// collection is stable rather than map-ordered.
type scope struct {
	provider *Provider
	name     string
	mu       sync.RWMutex
	order    []*instrument
	byName   map[string]*instrument
}

func (s *scope) instrument(name, unit, description string, kind InstrumentKind, monotonic bool, bounds []float64) *instrument {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.byName[name]; ok {
		return existing
	}
	created := &instrument{
		name: name, unit: unit, description: description, kind: kind,
		monotonic: monotonic, bounds: bounds, points: map[string]*point{},
	}
	s.byName[name] = created
	s.order = append(s.order, created)
	return created
}

func (s *scope) collect(temporality Temporality, start, now time.Time) []Data {
	s.mu.RLock()
	instruments := append([]*instrument(nil), s.order...)
	s.mu.RUnlock()
	var collected []Data
	for _, current := range instruments {
		if data, ok := current.collect(temporality, start, now); ok {
			collected = append(collected, data)
		}
	}
	return collected
}

// instrument is one metric's identity and its per-attribute-set accumulators.
type instrument struct {
	name        string
	unit        string
	description string
	kind        InstrumentKind
	monotonic   bool
	bounds      []float64

	mu     sync.Mutex
	order  []string
	points map[string]*point

	// observe is set for an observable instrument. It is called once per
	// collection and must return an already-computed value: it runs on the
	// reader's interval, and a callback that blocks blocks the export.
	observe func() []Observation
}

// point is one attribute set's accumulator.
type point struct {
	attributes []otel.Attribute
	// sum carries a counter's total and a histogram's sum of recorded values.
	sum          float64
	count        uint64
	min          float64
	max          float64
	bucketCounts []uint64
}

// Observation is one value an observable reports for one attribute set.
type Observation struct {
	Attributes []otel.Attribute
	Value      float64
}

// Meter creates instruments for one instrumentation scope. A zero Meter, which
// is what a nil provider returns, produces instruments that record nothing.
type Meter struct{ scope *scope }

// Counter is a monotonic total: requests served, cache hits, statements run.
type Counter struct{ instrument *instrument }

// UpDownCounter is a total that may fall, for a quantity that exists rather
// than accumulates: active requests, open connections, live subscriptions.
type UpDownCounter struct{ instrument *instrument }

// Histogram is a distribution, which is what makes a percentile answerable.
// Its count and sum also answer the questions a plain counter would, so a
// duration histogram needs no counter beside it.
type Histogram struct{ instrument *instrument }

// Observable is a value read at collection time rather than recorded as it
// happens, for a quantity something else already tracks — a pool's connection
// count, a store's counters, a runtime statistic.
type Observable struct{ instrument *instrument }

func (m *Meter) Counter(name, unit, description string) *Counter {
	if m == nil || m.scope == nil {
		return &Counter{}
	}
	return &Counter{instrument: m.scope.instrument(name, unit, description, KindCounter, true, nil)}
}

func (m *Meter) UpDownCounter(name, unit, description string) *UpDownCounter {
	if m == nil || m.scope == nil {
		return &UpDownCounter{}
	}
	return &UpDownCounter{instrument: m.scope.instrument(name, unit, description, KindUpDownCounter, false, nil)}
}

// Histogram creates a distribution over the given boundaries. Passing nil takes
// DurationBounds, since a duration is what most of them measure.
func (m *Meter) Histogram(name, unit, description string, bounds []float64) *Histogram {
	if m == nil || m.scope == nil {
		return &Histogram{}
	}
	if bounds == nil {
		bounds = DurationBounds
	}
	return &Histogram{instrument: m.scope.instrument(name, unit, description, KindHistogram, false, bounds)}
}

// ObservableCounter registers a monotonic total that is read at collection.
// Registering the same name twice replaces the callback, so a resource opened
// again does not report itself twice.
func (m *Meter) ObservableCounter(name, unit, description string, observe func() []Observation) *Observable {
	return m.observable(name, unit, description, KindCounter, true, observe)
}

// ObservableUpDownCounter registers a rising and falling total read at
// collection.
func (m *Meter) ObservableUpDownCounter(name, unit, description string, observe func() []Observation) *Observable {
	return m.observable(name, unit, description, KindUpDownCounter, false, observe)
}

// ObservableGauge registers a sampled value, for a quantity that is neither a
// total nor a distribution.
func (m *Meter) ObservableGauge(name, unit, description string, observe func() []Observation) *Observable {
	return m.observable(name, unit, description, KindGauge, false, observe)
}

func (m *Meter) observable(name, unit, description string, kind InstrumentKind, monotonic bool, observe func() []Observation) *Observable {
	if m == nil || m.scope == nil {
		return &Observable{}
	}
	created := m.scope.instrument(name, unit, description, kind, monotonic, nil)
	created.mu.Lock()
	created.observe = observe
	created.mu.Unlock()
	return &Observable{instrument: created}
}

// Add accumulates value. A nil receiver is the disabled case and costs one
// comparison.
func (c *Counter) Add(_ context.Context, value int64, attributes ...otel.Attribute) {
	if c == nil || c.instrument == nil {
		return
	}
	c.instrument.add(float64(value), attributes)
}

func (c *UpDownCounter) Add(_ context.Context, value int64, attributes ...otel.Attribute) {
	if c == nil || c.instrument == nil {
		return
	}
	c.instrument.add(float64(value), attributes)
}

// Record places value in its bucket. The context is accepted for the shape every
// other instrument API has and because an exemplar would need it; nothing reads
// it, which is deliberate — an exemplar would tie a metric back to a sampling
// decision the metric exists to be independent of.
func (h *Histogram) Record(_ context.Context, value float64, attributes ...otel.Attribute) {
	if h == nil || h.instrument == nil {
		return
	}
	h.instrument.record(value, attributes)
}

// Enabled reports whether recording reaches an aggregation, so a caller that
// would have to compute a value can skip it.
func (h *Histogram) Enabled() bool { return h != nil && h.instrument != nil }
func (c *Counter) Enabled() bool   { return c != nil && c.instrument != nil }

func (i *instrument) add(value float64, attributes []otel.Attribute) {
	key := attributeKey(attributes)
	i.mu.Lock()
	current := i.pointLocked(key, attributes)
	current.sum += value
	current.count++
	i.mu.Unlock()
}

func (i *instrument) record(value float64, attributes []otel.Attribute) {
	key := attributeKey(attributes)
	i.mu.Lock()
	current := i.pointLocked(key, attributes)
	if current.bucketCounts == nil {
		current.bucketCounts = make([]uint64, len(i.bounds)+1)
	}
	if current.count == 0 || value < current.min {
		current.min = value
	}
	if current.count == 0 || value > current.max {
		current.max = value
	}
	current.sum += value
	current.count++
	current.bucketCounts[bucketOf(i.bounds, value)]++
	i.mu.Unlock()
}

// pointLocked returns the accumulator for one attribute set, creating it in
// first-record order. Callers hold i.mu.
func (i *instrument) pointLocked(key string, attributes []otel.Attribute) *point {
	if existing, ok := i.points[key]; ok {
		return existing
	}
	created := &point{attributes: append([]otel.Attribute(nil), attributes...)}
	i.points[key] = created
	i.order = append(i.order, key)
	return created
}

func (i *instrument) collect(temporality Temporality, start, now time.Time) (Data, bool) {
	data := Data{
		Name: i.name, Unit: i.unit, Description: i.description,
		Kind: i.kind, Temporality: temporality, Monotonic: i.monotonic,
	}
	i.mu.Lock()
	observe := i.observe
	i.mu.Unlock()
	if observe != nil {
		// An observable holds no accumulator: the callback answers a value that
		// already exists, and for a counter that value is a total since process
		// start. Reporting it as a delta would claim that a running total was one
		// interval's worth, so an observable sum is always cumulative whatever
		// the reader's temporality is. A gauge carries no temporality at all.
		if data.Kind != KindGauge {
			data.Temporality = CumulativeTemporality
		}
		for _, observation := range observe() {
			data.Numbers = append(data.Numbers, NumberPoint{
				Attributes: observation.Attributes, Value: observation.Value, Start: start, Time: now,
			})
		}
		return data, len(data.Numbers) > 0
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, key := range i.order {
		current := i.points[key]
		if current.count == 0 && temporality == DeltaTemporality {
			// Nothing happened in this interval. Reporting a zero would make a
			// series that has stopped indistinguishable from one that is idle at
			// zero, and a delta reader averages over both.
			continue
		}
		switch i.kind {
		case KindHistogram:
			data.Histograms = append(data.Histograms, HistogramPoint{
				Attributes: current.attributes, Count: current.count, Sum: current.sum,
				Min: current.min, Max: current.max, Bounds: i.bounds,
				BucketCounts: append([]uint64(nil), current.bucketCounts...),
				Start:        start, Time: now,
			})
		default:
			data.Numbers = append(data.Numbers, NumberPoint{
				Attributes: current.attributes, Value: current.sum, Start: start, Time: now,
			})
		}
		if temporality == DeltaTemporality {
			current.sum, current.count, current.min, current.max = 0, 0, 0, 0
			for index := range current.bucketCounts {
				current.bucketCounts[index] = 0
			}
		}
	}
	return data, len(data.Numbers) > 0 || len(data.Histograms) > 0
}

// bucketOf returns the index of the first boundary value does not exceed, which
// is the OTLP convention: bucket i counts values in (bounds[i-1], bounds[i]].
func bucketOf(bounds []float64, value float64) int {
	return sort.SearchFloat64s(bounds, value)
}

// attributeKey identifies one attribute set.
//
// The set is sorted by key so that two callers passing the same attributes in
// different orders reach one series. One short string is built per recording;
// that is the cost of an attribute set that is not known until the call, and it
// is the place to look first if a hot path ever needs to be cheaper.
func attributeKey(attributes []otel.Attribute) string {
	switch len(attributes) {
	case 0:
		return ""
	case 1:
		return attributes[0].Key + "\x00" + attributeValue(attributes[0])
	}
	ordered := make([]int, len(attributes))
	for index := range attributes {
		ordered[index] = index
	}
	sort.Slice(ordered, func(a, b int) bool { return attributes[ordered[a]].Key < attributes[ordered[b]].Key })
	var builder strings.Builder
	for _, index := range ordered {
		builder.WriteString(attributes[index].Key)
		builder.WriteByte(0)
		builder.WriteString(attributeValue(attributes[index]))
		builder.WriteByte(0)
	}
	return builder.String()
}

func attributeValue(attribute otel.Attribute) string {
	if value, ok := attribute.Value.AsString(); ok {
		return value
	}
	if value, ok := attribute.Value.AsInt64(); ok {
		return strconv.FormatInt(value, 10)
	}
	if value, ok := attribute.Value.AsBool(); ok {
		return strconv.FormatBool(value)
	}
	if value, ok := attribute.Value.AsFloat64(); ok {
		return strconv.FormatFloat(value, 'g', -1, 64)
	}
	return ""
}

var (
	defaultMu       sync.RWMutex
	defaultProvider = NewProvider()
)

// DefaultProvider is what an instrument reaches when nothing was injected. It
// aggregates with no reader attached until one is installed, which is what lets
// a package take its meter at init and still be disabled.
func DefaultProvider() *Provider {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultProvider
}

func SetDefaultProvider(provider *Provider) {
	if provider == nil {
		provider = NewProvider()
	}
	defaultMu.Lock()
	defaultProvider = provider
	defaultMu.Unlock()
}
