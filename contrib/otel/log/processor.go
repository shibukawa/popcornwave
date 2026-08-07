package log

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

type Exporter interface {
	ExportLogs(context.Context, []RecordData) error
}

type SimpleProcessor struct {
	exporter Exporter
	mu       sync.Mutex
	err      error
}

func NewSimpleProcessor(exporter Exporter) *SimpleProcessor {
	return &SimpleProcessor{exporter: exporter}
}
func (p *SimpleProcessor) OnEmit(record RecordData) {
	if p == nil || p.exporter == nil {
		return
	}
	if err := p.exporter.ExportLogs(context.Background(), []RecordData{record}); err != nil {
		p.mu.Lock()
		p.err = errors.Join(p.err, err)
		p.mu.Unlock()
	}
}
func (p *SimpleProcessor) Shutdown(context.Context) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

type BatchConfig struct {
	QueueSize     int
	MaxExportSize int
	FlushInterval time.Duration
}
type BatchProcessor struct {
	exporter Exporter
	config   BatchConfig
	queue    chan RecordData
	done     chan struct{}
	mu       sync.RWMutex
	closing  bool
	dropped  atomic.Uint64
	err      error
}

func NewBatchProcessor(exporter Exporter, config BatchConfig) *BatchProcessor {
	if config.QueueSize <= 0 {
		config.QueueSize = 2048
	}
	if config.MaxExportSize <= 0 || config.MaxExportSize > config.QueueSize {
		config.MaxExportSize = 512
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 5 * time.Second
	}
	p := &BatchProcessor{exporter: exporter, config: config, queue: make(chan RecordData, config.QueueSize), done: make(chan struct{})}
	go p.run()
	return p
}
func (p *BatchProcessor) OnEmit(record RecordData) {
	if p == nil {
		return
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closing {
		p.dropped.Add(1)
		return
	}
	select {
	case p.queue <- record:
	default:
		p.dropped.Add(1)
	}
}
func (p *BatchProcessor) Dropped() uint64 {
	if p == nil {
		return 0
	}
	return p.dropped.Load()
}
func (p *BatchProcessor) Error() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}
func (p *BatchProcessor) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	if !p.closing {
		p.closing = true
		close(p.queue)
	}
	p.mu.Unlock()
	select {
	case <-p.done:
		return p.Error()
	case <-ctx.Done():
		return errors.Join(p.Error(), ctx.Err())
	}
}
func (p *BatchProcessor) run() {
	defer close(p.done)
	ticker := time.NewTicker(p.config.FlushInterval)
	defer ticker.Stop()
	batch := make([]RecordData, 0, p.config.MaxExportSize)
	flush := func() {
		if len(batch) == 0 || p.exporter == nil {
			batch = batch[:0]
			return
		}
		if err := p.exporter.ExportLogs(context.Background(), batch); err != nil {
			p.mu.Lock()
			p.err = errors.Join(p.err, err)
			p.dropped.Add(uint64(len(batch)))
			p.mu.Unlock()
		}
		batch = batch[:0]
	}
	for {
		select {
		case record, ok := <-p.queue:
			if !ok {
				flush()
				return
			}
			batch = append(batch, record)
			if len(batch) >= p.config.MaxExportSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
