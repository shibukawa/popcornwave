package metric

import (
	"context"
	"sync"
	"time"
)

// ReaderConfig bounds one periodic reader.
type ReaderConfig struct {
	// Interval is how often a collection is exported, and therefore how coarse
	// every chart of it becomes. It is a separate number from the trace batch
	// flush interval, because that one is how often a batch leaves and this one
	// is the resolution of the data.
	Interval time.Duration
	// Timeout bounds one export, including its retries.
	Timeout time.Duration
}

// Reader collects on an interval and exports each collection.
//
// It has no queue and drops nothing: an aggregation is bounded by its attribute
// sets rather than by traffic, so there is no per-record backpressure to apply.
// A failed export loses that interval, which under delta temporality is a
// permanent loss of those counts and under cumulative is repaired by the next
// export.
type Reader struct {
	provider *Provider
	exporter Exporter
	config   ReaderConfig
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once

	mu  sync.Mutex
	err error
}

const (
	defaultReaderInterval = 60 * time.Second
	defaultReaderTimeout  = 30 * time.Second
)

// NewReader starts the collection loop. A nil provider or exporter returns nil,
// so a disabled configuration produces no goroutine and no ticker.
func NewReader(provider *Provider, exporter Exporter, config ReaderConfig) *Reader {
	if provider == nil || exporter == nil {
		return nil
	}
	if config.Interval <= 0 {
		config.Interval = defaultReaderInterval
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultReaderTimeout
	}
	reader := &Reader{
		provider: provider, exporter: exporter, config: config,
		stop: make(chan struct{}), done: make(chan struct{}),
	}
	go reader.run()
	return reader
}

func (r *Reader) run() {
	defer close(r.done)
	// Not aligned to a wall-clock boundary: two instances that aligned to one
	// would export together, and a backend would see the sum arrive in bursts.
	ticker := time.NewTicker(r.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.export(context.Background())
		}
	}
}

// export performs one collection. It is exported through Collect for a test that
// needs a deterministic interval rather than a real one.
func (r *Reader) export(parent context.Context) {
	collected := r.provider.Collect()
	if len(collected.Scopes) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(parent, r.config.Timeout)
	defer cancel()
	if err := r.exporter.ExportMetrics(ctx, collected); err != nil {
		r.mu.Lock()
		r.err = err
		r.mu.Unlock()
	}
}

// Collect exports one collection now, outside the interval.
func (r *Reader) Collect(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.export(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// Shutdown stops the interval and exports once more within the caller's
// deadline, so the counts of the final interval are not the ones discarded.
func (r *Reader) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.once.Do(func() {
		close(r.stop)
		<-r.done
		r.export(ctx)
	})
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}
