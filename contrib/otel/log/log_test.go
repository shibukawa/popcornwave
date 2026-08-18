package log

import (
	"context"
	"testing"

	"github.com/shibukawa/popcornweb/contrib/otel/trace"
)

type recordCollector struct{ records []RecordData }

func (c *recordCollector) OnEmit(record RecordData)     { c.records = append(c.records, record) }
func (*recordCollector) Shutdown(context.Context) error { return nil }

func TestEmitCorrelatedAndStandalone(t *testing.T) {
	collector := &recordCollector{}
	logger := NewProvider(collector).Logger("test")
	logger.Emit(context.Background(), Record{Severity: SeverityInfo, Body: "standalone"})
	ctx, span := trace.NewProvider(nil).Tracer("test").Start(context.Background(), "request")
	logger.Emit(ctx, Record{Severity: SeverityError, Body: "correlated"})
	span.End()
	if len(collector.records) != 2 {
		t.Fatalf("records = %d", len(collector.records))
	}
	if collector.records[0].TraceID != "" || collector.records[0].SpanID != "" {
		t.Fatal("standalone log was correlated")
	}
	if collector.records[1].TraceID == "" || collector.records[1].SpanID == "" {
		t.Fatal("active trace was not correlated")
	}
}
