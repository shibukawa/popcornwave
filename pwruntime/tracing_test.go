package pwruntime

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/popcornwave/contrib/otel/trace"
)

// spanRecorder collects completed spans in memory. It replaces no global: the
// tracer reaches these tests through the root span's context, which is how a
// request reaches them in a served process too.
type spanRecorder struct {
	mu    sync.Mutex
	spans []trace.SpanData
}

func (recorder *spanRecorder) OnEnd(span trace.SpanData) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.spans = append(recorder.spans, span)
}

func (recorder *spanRecorder) Shutdown(context.Context) error { return nil }

func (recorder *spanRecorder) collected() []trace.SpanData {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]trace.SpanData(nil), recorder.spans...)
}

// named returns the spans carrying one name, so an assertion names what it
// wants rather than indexing into everything the render produced.
func (recorder *spanRecorder) named(name string) []trace.SpanData {
	var found []trace.SpanData
	for _, span := range recorder.collected() {
		if span.Name == name {
			found = append(found, span)
		}
	}
	return found
}

func spanAttributes(span trace.SpanData) map[string]any {
	values := make(map[string]any, len(span.Attributes))
	for _, attribute := range span.Attributes {
		values[attribute.Key] = attributeValue(attribute)
	}
	return values
}

// tracedContext installs a database, a recorder, and the root span every
// framework span is a child of.
func tracedContext(t *testing.T, query *QueryDiagnostics, tracing *Tracing) (context.Context, *spanRecorder, *captureHandler) {
	t.Helper()
	db, _ := newTestDB(t, "sqlite")
	handler := newCaptureHandler()
	recorder := &spanRecorder{}
	ctx := WithResources(context.Background(), Resources{
		DB: db, DBDriver: "sqlite", Query: query, Trace: tracing, Log: handler.backend(),
	})
	ctx, root := trace.NewProvider(recorder).Tracer("test").Start(ctx, "request")
	t.Cleanup(root.End)
	return ctx, recorder, handler
}

func databaseTracing() *Tracing {
	return &Tracing{Render: true, Boundary: true, Database: true, Statement: true, MaxSQLLength: 4096}
}

func TestDatabaseSpanDescribesTheStatement(t *testing.T) {
	ctx, recorder, _ := tracedContext(t, defaultDiagnostics(), databaseTracing())
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecContext(ctx, "INSERT INTO items (name) VALUES ($1)", "alpha"); err != nil {
		t.Fatal(err)
	}

	spans := recorder.named("INSERT")
	if len(spans) != 1 {
		t.Fatalf("want 1 INSERT span, got %d of %d spans", len(spans), len(recorder.collected()))
	}
	span := spans[0]
	if span.Kind != trace.SpanKindClient {
		t.Errorf("kind = %v, want client", span.Kind)
	}
	attributes := spanAttributes(span)
	if got := attributes["db.system.name"]; got != "sqlite" {
		t.Errorf("db.system.name = %v", got)
	}
	if got := attributes["db.operation.name"]; got != "INSERT" {
		t.Errorf("db.operation.name = %v", got)
	}
	if got := attributes["db.query.text"]; got != "INSERT INTO items (name) VALUES ($1)" {
		t.Errorf("db.query.text = %v", got)
	}
	if got := attributes["pw.db.rows_affected"]; got != int64(1) {
		t.Errorf("pw.db.rows_affected = %v, want 1", got)
	}
}

// TestDatabaseSpanCarriesNoBindValues is the boundary policy:query-log-safety
// draws: a trace is retained longer and read more widely than a log, so row
// data stays on the record the span id correlates.
func TestDatabaseSpanCarriesNoBindValues(t *testing.T) {
	ctx, recorder, handler := tracedContext(t, defaultDiagnostics(), databaseTracing())
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecContext(ctx, "INSERT INTO items (name) VALUES ($1)", "sensitive"); err != nil {
		t.Fatal(err)
	}
	for _, span := range recorder.collected() {
		for _, attribute := range span.Attributes {
			if value, ok := attribute.Value.AsString(); ok && value == "sensitive" {
				t.Fatalf("bind value reached span attribute %q", attribute.Key)
			}
		}
	}
	// The record still has it, so nothing was lost — only moved.
	if got := handler.only(t)["args"]; got != "sensitive" {
		t.Errorf("args = %v, want the value on the record", got)
	}
}

func TestDatabaseSpanOmitsStatementWhenNotWanted(t *testing.T) {
	tracing := databaseTracing()
	tracing.Statement = false
	ctx, recorder, _ := tracedContext(t, nil, tracing)
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecContext(ctx, "INSERT INTO items (name) VALUES ($1)", "alpha"); err != nil {
		t.Fatal(err)
	}
	spans := recorder.named("INSERT")
	if len(spans) != 1 {
		t.Fatalf("want 1 INSERT span, got %d", len(spans))
	}
	if _, present := spanAttributes(spans[0])["db.query.text"]; present {
		t.Error("statement text reached the span with observability.trace.statement off")
	}
}

func TestDatabaseSpanTruncatesTheStatement(t *testing.T) {
	tracing := databaseTracing()
	tracing.MaxSQLLength = 10
	ctx, recorder, _ := tracedContext(t, nil, tracing)
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecContext(ctx, "INSERT INTO items (name) VALUES ($1)", "alpha"); err != nil {
		t.Fatal(err)
	}
	attributes := spanAttributes(recorder.named("INSERT")[0])
	if got := attributes["db.query.text"]; got != "INSERT INT" {
		t.Errorf("db.query.text = %v, want the bounded prefix", got)
	}
	if got := attributes["pw.db.query_truncated"]; got != true {
		t.Errorf("pw.db.query_truncated = %v, want true", got)
	}
}

// TestDatabaseSpanWithoutQueryLog covers the setting a deployment usually
// wants: the span, which is bounded and lands beside the request, without a
// record per statement.
func TestDatabaseSpanWithoutQueryLog(t *testing.T) {
	ctx, recorder, handler := tracedContext(t, nil, databaseTracing())
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.QueryContext(ctx, "SELECT name FROM items"); err != nil {
		t.Fatal(err)
	}
	if len(recorder.named("SELECT")) != 1 {
		t.Fatalf("want the statement span, got %d spans", len(recorder.collected()))
	}
	if records := handler.queries(); len(records) != 0 {
		t.Fatalf("query log wrote %d records with diagnostics off", len(records))
	}
}

func TestDatabaseSpanReportsFailure(t *testing.T) {
	ctx, recorder, _ := tracedContext(t, nil, databaseTracing())
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecContext(ctx, "INSERT INTO missing (name) VALUES ($1)", "alpha"); err == nil {
		t.Fatal("want the statement to fail")
	}
	spans := recorder.named("INSERT")
	if len(spans) != 1 {
		t.Fatalf("want 1 INSERT span, got %d", len(spans))
	}
	if spans[0].Status != trace.StatusError {
		t.Errorf("status = %v, want error", spans[0].Status)
	}
	if len(spans[0].Events) == 0 {
		t.Error("a failed statement recorded no exception event")
	}
}

func TestDatabaseSpanMarksSlowStatements(t *testing.T) {
	diagnostics := defaultDiagnostics()
	diagnostics.SlowThreshold = time.Nanosecond
	diagnostics.Explain = false
	diagnostics.Reproduction = false
	ctx, recorder, _ := tracedContext(t, diagnostics, databaseTracing())
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.QueryContext(ctx, "SELECT name FROM items"); err != nil {
		t.Fatal(err)
	}
	if got := spanAttributes(recorder.named("SELECT")[0])["pw.db.slow"]; got != true {
		t.Errorf("pw.db.slow = %v, want true", got)
	}
}

// TestNoDatabaseSpanWhenTracingOff also covers the cost claim: with neither
// setting on, the resolved executor is the bare pool.
func TestNoDatabaseSpanWhenTracingOff(t *testing.T) {
	ctx, recorder, _ := tracedContext(t, nil, nil)
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := executor.(*instrumentedExecutor); ok {
		t.Fatal("an untraced, unlogged executor was still wrapped")
	}
	if _, err := executor.ExecContext(ctx, "INSERT INTO items (name) VALUES ($1)", "alpha"); err != nil {
		t.Fatal(err)
	}
	if spans := recorder.collected(); len(spans) != 0 {
		t.Fatalf("want no span, got %d", len(spans))
	}
}

// TestDatabaseSpanCorrelatesTheRecord ties the two surfaces together: the
// record of a statement names the span of that statement, so a waterfall entry
// leads to the values, the plan, and the rerun snippet.
func TestDatabaseSpanCorrelatesTheRecord(t *testing.T) {
	ctx, recorder, handler := tracedContext(t, defaultDiagnostics(), databaseTracing())
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.QueryContext(ctx, "SELECT name FROM items"); err != nil {
		t.Fatal(err)
	}
	spans := recorder.named("SELECT")
	if len(spans) != 1 {
		t.Fatalf("want 1 SELECT span, got %d", len(spans))
	}
	records := handler.sink.Records()
	if len(records) != 1 {
		t.Fatalf("want 1 record, got %d", len(records))
	}
	if records[0].SpanID != spans[0].SpanContext.SpanID() {
		t.Errorf("record span id = %q, want the statement span %q", records[0].SpanID, spans[0].SpanContext.SpanID())
	}
}

func TestSQLOperationReadsTheLeadingKeyword(t *testing.T) {
	for _, testCase := range []struct{ query, want string }{
		{"SELECT 1", "SELECT"},
		{"  \n select * from items", "SELECT"},
		{"insert into items(name) values($1)", "INSERT"},
		{"WITH recent AS (SELECT 1) SELECT * FROM recent", "WITH"},
		{"-- a generated comment\nUPDATE items SET name = $1", "UPDATE"},
		{"/* leading block */ DELETE FROM items", "DELETE"},
		{"EXPLAIN QUERY PLAN SELECT 1", "EXPLAIN"},
		// Anything the allowlist does not name falls back to the executor
		// operation rather than putting an unknown word into a span name.
		{"$$ not sql $$", ""},
		{"", ""},
	} {
		if got := sqlOperation(testCase.query); got != testCase.want {
			t.Errorf("sqlOperation(%q) = %q, want %q", testCase.query, got, testCase.want)
		}
	}
}

func TestUnnamedStatementFallsBackToTheOperation(t *testing.T) {
	ctx, recorder, _ := tracedContext(t, nil, databaseTracing())
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// A statement the allowlist does not name still fails, and the span still
	// has to be named something bounded.
	_, _ = executor.QueryContext(ctx, "$$ not sql $$")
	if len(recorder.named("query")) != 1 {
		t.Fatalf("want the fallback span name, got %v", recorder.collected())
	}
}
