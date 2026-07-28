package pwruntime

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shibukawa/tinybind-go/sqlbind"
)

// captureHandler collects emitted records so a test can assert on attributes
// instead of on formatted output.
type captureHandler struct {
	mu      sync.Mutex
	level   slog.Level
	records []map[string]any
}

func (handler *captureHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= handler.level
}

func (handler *captureHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := map[string]any{"@message": record.Message, "@level": record.Level}
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()
		return true
	})
	handler.mu.Lock()
	handler.records = append(handler.records, attrs)
	handler.mu.Unlock()
	return nil
}

func (handler *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return handler }
func (handler *captureHandler) WithGroup(string) slog.Handler      { return handler }

func (handler *captureHandler) queries() []map[string]any {
	handler.mu.Lock()
	defer handler.mu.Unlock()
	var found []map[string]any
	for _, record := range handler.records {
		if record["@message"] == queryMessage {
			found = append(found, record)
		}
	}
	return found
}

func (handler *captureHandler) only(t *testing.T) map[string]any {
	t.Helper()
	found := handler.queries()
	if len(found) != 1 {
		t.Fatalf("want 1 query record, got %d: %v", len(found), found)
	}
	return found[0]
}

// diagnosticContext builds a database and a context instrumented with config.
func diagnosticContext(t *testing.T, driver string, config *QueryDiagnostics) (context.Context, *captureHandler) {
	t.Helper()
	db, _ := newTestDB(t, driver)
	handler := &captureHandler{level: LevelTrace}
	base := Resources{DB: db, DBDriver: driver, Query: config, Logger: slog.New(handler)}
	return WithResources(context.Background(), base), handler
}

func defaultDiagnostics() *QueryDiagnostics {
	return &QueryDiagnostics{
		Level:          slog.LevelInfo,
		SlowLevel:      slog.LevelWarn,
		SlowThreshold:  time.Hour,
		BindValues:     true,
		Explain:        true,
		Reproduction:   true,
		MaxSQLLength:   4096,
		MaxValueLength: 256,
	}
}

func TestSQLExecutorLeavesExecutorBareWhenDisabled(t *testing.T) {
	db, ctx := newTestDB(t, "sqlite")
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if executor != sqlbind.SQLExecutor(db) {
		t.Fatalf("want the bare pool, got %T", executor)
	}
}

func TestQueryLogRecordsStatement(t *testing.T) {
	ctx, handler := diagnosticContext(t, "sqlite", defaultDiagnostics())
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecContext(ctx, "INSERT INTO items (name) VALUES ($1)", "alpha"); err != nil {
		t.Fatal(err)
	}

	record := handler.only(t)
	if record["@level"] != slog.LevelInfo {
		t.Errorf("level = %v, want info", record["@level"])
	}
	if got := record["sql"]; got != "INSERT INTO items (name) VALUES ($1)" {
		t.Errorf("sql = %v", got)
	}
	if got := record["operation"]; got != "exec" {
		t.Errorf("operation = %v, want exec", got)
	}
	if got := record["outcome"]; got != "ok" {
		t.Errorf("outcome = %v, want ok", got)
	}
	if got := record["rows_affected"]; got != int64(1) {
		t.Errorf("rows_affected = %v, want 1", got)
	}
	if got := record["driver"]; got != "sqlite" {
		t.Errorf("driver = %v, want sqlite", got)
	}
	args, ok := record["args"].([]string)
	if !ok || len(args) != 1 || args[0] != "alpha" {
		t.Errorf("args = %v", record["args"])
	}
	if _, present := record["slow"]; present {
		t.Errorf("statement under the threshold was marked slow")
	}
	if _, present := record["tx_depth"]; present {
		t.Errorf("statement outside a transaction reported a depth")
	}
}

func TestQueryLogOmitsValuesWhenBindValuesOff(t *testing.T) {
	config := defaultDiagnostics()
	config.BindValues = false
	ctx, handler := diagnosticContext(t, "sqlite", config)
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecContext(ctx, "INSERT INTO items (name) VALUES ($1)", "secret"); err != nil {
		t.Fatal(err)
	}
	record := handler.only(t)
	if _, present := record["args"]; present {
		t.Errorf("bind values leaked with bind_values off: %v", record["args"])
	}
	if record["sql"] == nil {
		t.Errorf("statement text should survive bind_values off")
	}
}

func TestQueryLogRecordsFailure(t *testing.T) {
	ctx, handler := diagnosticContext(t, "sqlite", defaultDiagnostics())
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecContext(ctx, "INSERT INTO missing (name) VALUES ($1)", "alpha"); err == nil {
		t.Fatal("want the statement to fail")
	}
	record := handler.only(t)
	if got := record["outcome"]; got != "error" {
		t.Errorf("outcome = %v, want error", got)
	}
	if record["error"] == nil {
		t.Errorf("failed statement recorded no error")
	}
}

func TestSlowQueryExplainsAndReproduces(t *testing.T) {
	config := defaultDiagnostics()
	config.SlowThreshold = time.Nanosecond
	ctx, handler := diagnosticContext(t, "sqlite", config)
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := executor.QueryContext(ctx, "SELECT name FROM items WHERE name = $1", "alpha")
	if err != nil {
		t.Fatal(err)
	}
	_ = rows.Close()

	record := handler.only(t)
	if record["@level"] != slog.LevelWarn {
		t.Errorf("level = %v, want warn", record["@level"])
	}
	if record["slow"] != true {
		t.Errorf("slow = %v, want true", record["slow"])
	}
	plan, _ := record["explain"].(string)
	if !strings.Contains(plan, "items") {
		t.Errorf("explain = %q, want a plan naming the table", plan)
	}
	if record["explain_error"] != nil {
		t.Errorf("explain_error = %v", record["explain_error"])
	}
	snippet, _ := record["reproduction"].(string)
	if !strings.Contains(snippet, ".parameter set $1 'alpha'") {
		t.Errorf("reproduction = %q", snippet)
	}
	if !strings.Contains(snippet, "SELECT name FROM items WHERE name = $1;") {
		t.Errorf("reproduction lost the statement: %q", snippet)
	}
}

func TestSlowQuerySkipsExplainOnUnsupportedDriver(t *testing.T) {
	config := defaultDiagnostics()
	config.SlowThreshold = time.Nanosecond
	ctx, handler := diagnosticContext(t, "cockroach", config)
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecContext(ctx, "INSERT INTO items (name) VALUES ($1)", "alpha"); err != nil {
		t.Fatal(err)
	}
	record := handler.only(t)
	if record["slow"] != true {
		t.Errorf("the query log should survive an unsupported driver")
	}
	if record["explain"] != nil || record["explain_error"] != nil {
		t.Errorf("unsupported driver produced a plan: %v %v", record["explain"], record["explain_error"])
	}
	if record["reproduction"] != nil {
		t.Errorf("unsupported driver produced a snippet: %v", record["reproduction"])
	}
}

func TestQueryLogTruncatesStatementAndValues(t *testing.T) {
	config := defaultDiagnostics()
	config.SlowThreshold = time.Nanosecond
	config.MaxSQLLength = 10
	config.MaxValueLength = 3
	ctx, handler := diagnosticContext(t, "sqlite", config)
	executor, err := SQLExecutor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executor.ExecContext(ctx, "INSERT INTO items (name) VALUES ($1)", "alphabet"); err != nil {
		t.Fatal(err)
	}
	record := handler.only(t)
	if got := record["sql"]; got != "INSERT INT" {
		t.Errorf("sql = %v, want the bounded prefix", got)
	}
	if record["sql_truncated"] != true || record["args_truncated"] != true {
		t.Errorf("truncation went unreported: %v", record)
	}
	if record["reproduction"] != nil {
		t.Errorf("a truncated value must not produce a rerun snippet: %v", record["reproduction"])
	}
}

func TestQueryLogReportsTransactionDepth(t *testing.T) {
	ctx, handler := diagnosticContext(t, "sqlite", defaultDiagnostics())
	err := Transaction(ctx, func(ctx context.Context) error {
		return Transaction(ctx, func(ctx context.Context) error {
			executor, err := SQLExecutor(ctx)
			if err != nil {
				return err
			}
			_, err = executor.ExecContext(ctx, "INSERT INTO items (name) VALUES ($1)", "nested")
			return err
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	record := handler.only(t)
	if got := record["tx_depth"]; got != int64(1) {
		t.Errorf("tx_depth = %v, want 1", got)
	}
}

// Instrumentation must not hide the transaction handle that nesting asserts on,
// or an adopted transaction would silently become a second one on the pool.
func TestTransactionAdoptsInstrumentedExecutor(t *testing.T) {
	db, _ := newTestDB(t, "sqlite")
	handler := &captureHandler{level: LevelTrace}
	config := defaultDiagnostics()
	ctx := WithResources(context.Background(), Resources{
		DB: db, DBDriver: "sqlite", Query: config, Logger: slog.New(handler),
	})

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	wrapped := &instrumentedExecutor{inner: tx, config: config, logger: slog.New(handler), driver: "sqlite"}
	ctx = sqlbind.WithSQLExecutor(ctx, wrapped)

	if err := Transaction(ctx, func(ctx context.Context) error {
		executor, err := SQLExecutor(ctx)
		if err != nil {
			return err
		}
		_, err = executor.ExecContext(ctx, "INSERT INTO items (name) VALUES ($1)", "adopted")
		return err
	}); err != nil {
		t.Fatal(err)
	}

	// Visible inside the adopted transaction, gone once it rolls back: proof the
	// insert joined that transaction instead of opening its own.
	var count int
	if err := tx.QueryRow("SELECT count(*) FROM items WHERE name = 'adopted'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("row count inside the adopted transaction = %d, want 1", count)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if names := names(t, db); len(names) != 0 {
		t.Fatalf("rollback left %v behind", names)
	}
}

func TestSupportsExplain(t *testing.T) {
	for driver, want := range map[string]bool{
		"sqlite": true, "postgres": true, "pgx": true, "mysql": true,
		"cockroach": false, "": false,
	} {
		if got := SupportsExplain(driver); got != want {
			t.Errorf("SupportsExplain(%q) = %v, want %v", driver, got, want)
		}
	}
}
