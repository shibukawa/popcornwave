package metric

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/contrib/otel"
)

type collector struct {
	mu          sync.Mutex
	collections []ResourceData
}

func (c *collector) ExportMetrics(_ context.Context, data ResourceData) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.collections = append(c.collections, data)
	return nil
}

func (c *collector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.collections)
}

// find returns one instrument's collected data from the last collection.
func find(t *testing.T, data ResourceData, name string) Data {
	t.Helper()
	for _, scope := range data.Scopes {
		for _, metric := range scope.Metrics {
			if metric.Name == name {
				return metric
			}
		}
	}
	t.Fatalf("%s is not in the collection", name)
	return Data{}
}

func TestCounterAndHistogramAggregatePerAttributeSet(t *testing.T) {
	provider := NewProvider(WithResourceAttributes(otel.String("service.name", "test")))
	meter := provider.Meter("test-scope")
	requests := meter.Counter("http.server.request.count", "{request}", "")
	duration := meter.Histogram("http.server.request.duration", "s", "", DurationBounds)
	ctx := context.Background()
	ok := otel.Int64("http.response.status_code", 200)
	bad := otel.Int64("http.response.status_code", 500)
	requests.Add(ctx, 1, ok)
	requests.Add(ctx, 1, ok)
	requests.Add(ctx, 1, bad)
	duration.Record(ctx, 0.002, ok)
	duration.Record(ctx, 0.3, ok)

	collected := provider.Collect()
	if got := len(collected.ResourceAttributes); got != 1 {
		t.Fatalf("resource attributes = %d", got)
	}
	counts := find(t, collected, "http.server.request.count")
	if len(counts.Numbers) != 2 {
		t.Fatalf("series = %d, want one per status", len(counts.Numbers))
	}
	if counts.Numbers[0].Value != 2 || counts.Numbers[1].Value != 1 {
		t.Fatalf("values = %v and %v", counts.Numbers[0].Value, counts.Numbers[1].Value)
	}
	if !counts.Monotonic {
		t.Error("a counter is not reported as monotonic")
	}
	durations := find(t, collected, "http.server.request.duration")
	if len(durations.Histograms) != 1 {
		t.Fatalf("histogram series = %d", len(durations.Histograms))
	}
	point := durations.Histograms[0]
	if point.Count != 2 {
		t.Errorf("count = %d", point.Count)
	}
	if point.Sum != 0.302 {
		t.Errorf("sum = %v", point.Sum)
	}
	if point.Min != 0.002 || point.Max != 0.3 {
		t.Errorf("min/max = %v/%v", point.Min, point.Max)
	}
	// 0.002 falls in the first bucket, 0.3 in the one bounded by 0.5.
	if point.BucketCounts[0] != 1 {
		t.Errorf("first bucket = %d", point.BucketCounts[0])
	}
	if total := sum(point.BucketCounts); total != 2 {
		t.Errorf("bucket total = %d, want the count", total)
	}
}

func sum(counts []uint64) uint64 {
	var total uint64
	for _, count := range counts {
		total += count
	}
	return total
}

func TestAttributeOrderDoesNotSplitASeries(t *testing.T) {
	provider := NewProvider()
	counter := provider.Meter("test-scope").Counter("pw.test.count", "{item}", "")
	ctx := context.Background()
	counter.Add(ctx, 1, otel.String("a", "1"), otel.String("b", "2"))
	counter.Add(ctx, 1, otel.String("b", "2"), otel.String("a", "1"))
	data := find(t, provider.Collect(), "pw.test.count")
	if len(data.Numbers) != 1 {
		t.Fatalf("series = %d, want one", len(data.Numbers))
	}
	if data.Numbers[0].Value != 2 {
		t.Fatalf("value = %v", data.Numbers[0].Value)
	}
}

func TestDeltaTemporalityResetsAndSkipsIdleSeries(t *testing.T) {
	provider := NewProvider(WithTemporality(DeltaTemporality))
	counter := provider.Meter("test-scope").Counter("pw.test.count", "{item}", "")
	counter.Add(context.Background(), 5)
	if got := find(t, provider.Collect(), "pw.test.count").Numbers[0].Value; got != 5 {
		t.Fatalf("first collection = %v", got)
	}
	// Nothing happened since, so the series is absent rather than zero: a
	// stopped series and one idle at zero are different facts.
	if scopes := provider.Collect().Scopes; len(scopes) != 0 {
		t.Fatalf("an idle delta collection reported %d scopes", len(scopes))
	}
	counter.Add(context.Background(), 2)
	if got := find(t, provider.Collect(), "pw.test.count").Numbers[0].Value; got != 2 {
		t.Fatalf("second collection = %v, want the interval rather than the total", got)
	}
}

func TestCumulativeTemporalityKeepsTheTotal(t *testing.T) {
	provider := NewProvider(WithTemporality(CumulativeTemporality))
	counter := provider.Meter("test-scope").Counter("pw.test.count", "{item}", "")
	counter.Add(context.Background(), 5)
	provider.Collect()
	counter.Add(context.Background(), 2)
	if got := find(t, provider.Collect(), "pw.test.count").Numbers[0].Value; got != 7 {
		t.Fatalf("second collection = %v, want the running total", got)
	}
}

func TestObservableIsReadAtCollection(t *testing.T) {
	provider := NewProvider()
	value := int64(3)
	provider.Meter("test-scope").ObservableUpDownCounter("pw.test.open", "{connection}", "", func() []Observation {
		return []Observation{{Attributes: []otel.Attribute{otel.String("state", "used")}, Value: float64(value)}}
	})
	if got := find(t, provider.Collect(), "pw.test.open").Numbers[0].Value; got != 3 {
		t.Fatalf("first read = %v", got)
	}
	value = 9
	if got := find(t, provider.Collect(), "pw.test.open").Numbers[0].Value; got != 9 {
		t.Fatalf("second read = %v, want the value at collection time", got)
	}
}

func TestDisabledInstrumentsRecordNothing(t *testing.T) {
	var provider *Provider
	meter := provider.Meter("test-scope")
	counter := meter.Counter("pw.test.count", "{item}", "")
	histogram := meter.Histogram("pw.test.duration", "s", "", nil)
	if counter.Enabled() || histogram.Enabled() {
		t.Fatal("an instrument from a nil provider reports itself enabled")
	}
	// The point of the nil checks: a disabled build calls these without a
	// provider and must not panic.
	counter.Add(context.Background(), 1)
	histogram.Record(context.Background(), 1)
	(*Counter)(nil).Add(context.Background(), 1)
	(*Histogram)(nil).Record(context.Background(), 1)
	(*UpDownCounter)(nil).Add(context.Background(), 1)
}

func TestReaderExportsOnShutdown(t *testing.T) {
	provider := NewProvider(WithTemporality(DeltaTemporality))
	received := &collector{}
	reader := NewReader(provider, received, ReaderConfig{Interval: time.Hour})
	provider.Meter("test-scope").Counter("pw.test.count", "{item}", "").Add(context.Background(), 1)
	if got := received.count(); got != 0 {
		t.Fatalf("collections before shutdown = %d", got)
	}
	if err := reader.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := received.count(); got != 1 {
		t.Fatalf("collections after shutdown = %d, want the final interval", got)
	}
	// Shutting down twice must not export twice.
	if err := reader.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := received.count(); got != 1 {
		t.Fatalf("collections after a second shutdown = %d", got)
	}
}

func TestReaderCollectsOnItsInterval(t *testing.T) {
	provider := NewProvider(WithTemporality(DeltaTemporality))
	received := &collector{}
	reader := NewReader(provider, received, ReaderConfig{Interval: 5 * time.Millisecond})
	defer reader.Shutdown(context.Background())
	counter := provider.Meter("test-scope").Counter("pw.test.count", "{item}", "")
	deadline := time.Now().Add(2 * time.Second)
	for received.count() == 0 && time.Now().Before(deadline) {
		counter.Add(context.Background(), 1)
		time.Sleep(time.Millisecond)
	}
	if received.count() == 0 {
		t.Fatal("the reader never exported on its own interval")
	}
}

func TestNilReaderIsTheDisabledCase(t *testing.T) {
	if reader := NewReader(nil, &collector{}, ReaderConfig{}); reader != nil {
		t.Fatal("a nil provider produced a reader")
	}
	if reader := NewReader(NewProvider(), nil, ReaderConfig{}); reader != nil {
		t.Fatal("a nil exporter produced a reader")
	}
	if err := (*Reader)(nil).Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}
