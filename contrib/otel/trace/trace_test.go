package trace

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/contrib/otel"
)

type spanCollector struct{ spans []SpanData }

func (c *spanCollector) OnEnd(span SpanData)          { c.spans = append(c.spans, span) }
func (*spanCollector) Shutdown(context.Context) error { return nil }

func TestParentAndChildSpans(t *testing.T) {
	collector := &spanCollector{}
	tracer := NewProvider(collector, WithResourceAttributes(otel.String("service.name", "test"))).Tracer("test-scope")
	ctx, parent := tracer.Start(context.Background(), "parent")
	childCtx, child := Start(ctx, "child", WithAttributes(otel.String("db.system.name", "sqlite")))
	if SpanContextFromContext(childCtx).TraceID() != SpanContextFromContext(ctx).TraceID() {
		t.Fatal("child has a different trace ID")
	}
	child.End()
	parent.End()
	parent.End()
	if len(collector.spans) != 2 {
		t.Fatalf("completed spans = %d", len(collector.spans))
	}
	if collector.spans[0].ParentSpanID != SpanContextFromContext(ctx).SpanID() {
		t.Fatalf("parent span ID = %q", collector.spans[0].ParentSpanID)
	}
	if got := collector.spans[0].ResourceAttributes[0].Key; got != "service.name" {
		t.Fatalf("resource key = %q", got)
	}
}

type blockingExporter struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (e *blockingExporter) ExportSpans(context.Context, []SpanData) error {
	e.once.Do(func() { close(e.started) })
	<-e.release
	return nil
}

func TestBatchProcessorDropsWithoutBlocking(t *testing.T) {
	exporter := &blockingExporter{started: make(chan struct{}), release: make(chan struct{})}
	processor := NewBatchProcessor(exporter, BatchConfig{QueueSize: 1, MaxExportSize: 1, FlushInterval: time.Hour})
	processor.OnEnd(SpanData{Name: "exporting"})
	<-exporter.started
	processor.OnEnd(SpanData{Name: "queued"})
	processor.OnEnd(SpanData{Name: "dropped"})
	if got := processor.Dropped(); got != 1 {
		t.Fatalf("dropped = %d", got)
	}
	close(exporter.release)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := processor.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestEmptySpanContextIDs(t *testing.T) {
	if got := (SpanContext{}).TraceID(); got != "" {
		t.Fatalf("TraceID = %q", got)
	}
	if got := (SpanContext{}).SpanID(); got != "" {
		t.Fatalf("SpanID = %q", got)
	}
}
