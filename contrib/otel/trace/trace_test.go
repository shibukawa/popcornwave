package trace

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/popcornweb/contrib/otel"
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

func TestSpanParentPointerChain(t *testing.T) {
	tracer := NewProvider(&spanCollector{}).Tracer("test-scope")
	ctx, root := tracer.Start(context.Background(), "root")
	ctx, middle := Start(ctx, "middle")
	ctx, leaf := Start(ctx, "leaf")
	if got := SpanFromContext(ctx); got != leaf {
		t.Fatal("SpanFromContext is not the innermost span")
	}
	if leaf.Parent() != middle || middle.Parent() != root {
		t.Fatal("parent pointers do not follow the start order")
	}
	if root.Parent() != nil {
		t.Fatal("root span has a parent")
	}
	if leaf.Root() != root || root.Root() != root {
		t.Fatal("Root does not reach the outermost span")
	}
	if (*Span)(nil).Parent() != nil || (*Span)(nil).Root() != nil {
		t.Fatal("nil span accessors are not nil-safe")
	}
}

func TestSpanParentPointerAcrossRemoteParent(t *testing.T) {
	remote, err := NewSpanContext("0af7651916cd43dd8448eb211c80319c", "b7ad6b7169203331", 1, "", true)
	if err != nil {
		t.Fatal(err)
	}
	ctx := ContextWithSpanContext(context.Background(), remote)
	if SpanFromContext(ctx) != nil {
		t.Fatal("a propagation-only context reports a local span")
	}
	ctx, child := Start(ctx, "child")
	if child.Parent() != nil {
		t.Fatal("a remote parent has no local span to point at")
	}
	if got := SpanFromContext(ctx); got != child {
		t.Fatal("SpanFromContext is not the started span")
	}
	if got := child.SpanContext().TraceID(); got != remote.TraceID() {
		t.Fatalf("trace ID = %q", got)
	}
}
