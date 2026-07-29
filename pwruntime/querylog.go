package pwruntime

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shibukawa/tinybind-go/sqlbind"
)

// queryMessage is the stable message of a query diagnostics record.
const queryMessage = "sql executed"

// QueryDiagnostics is the resolved query diagnostics setting installed by pw.
//
// A nil value disables the feature entirely, so SQLExecutor returns the bare
// database handle and no statement is timed.
type QueryDiagnostics struct {
	// Level is the severity of an ordinary statement record.
	Level Level
	// SlowLevel is the severity once a statement exceeds SlowThreshold.
	SlowLevel Level
	// SlowThreshold is the duration above which a statement is slow. Zero
	// disables slow detection, and with it EXPLAIN and reproduction.
	SlowThreshold time.Duration
	// BindValues allows argument values into the record. It is the only path by
	// which application row data reaches a framework SQL log.
	BindValues bool
	// Explain captures a plan-only EXPLAIN for a slow statement.
	Explain bool
	// Reproduction renders a paste-able rerun snippet for a slow statement.
	Reproduction bool
	// MaxSQLLength bounds the logged statement text.
	MaxSQLLength int
	// MaxValueLength bounds each logged argument value.
	MaxValueLength int
}

// unwrapper is implemented by an executor that decorates another one.
type unwrapper interface {
	Unwrap() sqlbind.SQLExecutor
}

// unwrapExecutor returns the executor underneath any instrumentation, so a
// caller that needs the concrete transaction handle can reach past it.
func unwrapExecutor(executor sqlbind.SQLExecutor) sqlbind.SQLExecutor {
	for {
		wrapper, ok := executor.(unwrapper)
		if !ok {
			return executor
		}
		inner := wrapper.Unwrap()
		if inner == nil {
			return executor
		}
		executor = inner
	}
}

// instrument decorates executor when diagnostics are enabled. The wrapper is
// built once per executor resolution rather than once per statement.
func instrument(current *Resources, executor sqlbind.SQLExecutor, logger Logger) sqlbind.SQLExecutor {
	config := current.Query
	if config == nil || executor == nil {
		return executor
	}
	inTx, depth := current.TxScope.state()
	driver, label := current.DBDriver, ""
	if connection, err := current.connection(); err == nil {
		driver = connection.Driver
		// One connection needs no label: it would repeat on every record and
		// name the only database there is.
		if len(current.Connections.Connections()) > 1 {
			label = connection.Label
		}
	}
	return &instrumentedExecutor{
		inner:      executor,
		config:     config,
		logger:     logger,
		driver:     driver,
		connection: label,
		inTx:       inTx,
		depth:      depth,
	}
}

// instrumentedExecutor observes one statement per call without changing what
// the wrapped executor does with it.
type instrumentedExecutor struct {
	inner  sqlbind.SQLExecutor
	config *QueryDiagnostics
	logger Logger
	driver string
	// connection labels which pool ran the statement. Empty when only one is
	// configured.
	connection string
	inTx       bool
	depth      int
}

// Unwrap returns the observed executor.
func (executor *instrumentedExecutor) Unwrap() sqlbind.SQLExecutor { return executor.inner }

func (executor *instrumentedExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := executor.inner.ExecContext(ctx, query, args...)
	elapsed := time.Since(start)
	affected := int64(-1)
	if err == nil && result != nil {
		if value, affectedErr := result.RowsAffected(); affectedErr == nil {
			affected = value
		}
	}
	executor.record(ctx, "exec", query, args, elapsed, affected, err)
	return result, err
}

func (executor *instrumentedExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := executor.inner.QueryContext(ctx, query, args...)
	// The executor contract returns a concrete *sql.Rows, so there is nothing to
	// decorate: the row count and the scan time belong to the caller's loop and
	// are deliberately outside this measurement.
	executor.record(ctx, "query", query, args, time.Since(start), -1, err)
	return rows, err
}

// record emits at most one log record for one execution.
func (executor *instrumentedExecutor) record(ctx context.Context, operation, query string, args []any, elapsed time.Duration, affected int64, callErr error) {
	config := executor.config
	slow := config.SlowThreshold > 0 && elapsed >= config.SlowThreshold
	level := config.Level
	if slow {
		level = config.SlowLevel
	}
	if !executor.logger.Enabled(level) {
		return
	}

	statement, statementTruncated := truncateText(query, config.MaxSQLLength)
	attrs := []Attribute{
		String("sql", statement),
		Duration("duration", elapsed),
		String("operation", operation),
	}
	if statementTruncated {
		attrs = append(attrs, Bool("sql_truncated", true))
	}
	if executor.driver != "" {
		attrs = append(attrs, String("driver", executor.driver))
	}
	if executor.connection != "" {
		attrs = append(attrs, String("connection", executor.connection))
	}
	if executor.inTx {
		attrs = append(attrs, Int("tx_depth", executor.depth))
	}
	if affected >= 0 {
		attrs = append(attrs, Int64("rows_affected", affected))
	}
	if callErr != nil {
		attrs = append(attrs, String("outcome", "error"), String("error", callErr.Error()))
	} else {
		attrs = append(attrs, String("outcome", "ok"))
	}

	rendered, valuesComplete := renderArgs(args, config)
	if config.BindValues && len(args) > 0 {
		// One joined string rather than a list, because a record attribute is a
		// scalar: a consumer that wants the values separately reads the
		// reproduction snippet, which is the runnable form anyway.
		attrs = append(attrs, String("args", strings.Join(rendered, ", ")))
		if !valuesComplete {
			attrs = append(attrs, Bool("args_truncated", true))
		}
	}

	if slow {
		attrs = append(attrs, Bool("slow", true))
		if config.Explain {
			plan, planErr := executor.explain(ctx, query, args)
			switch {
			case planErr != nil:
				attrs = append(attrs, String("explain_error", planErr.Error()))
			case plan != "":
				attrs = append(attrs, String("explain", plan))
			}
		}
		// A snippet without its arguments cannot run, and a snippet built from a
		// truncated value reproduces a different query.
		if config.Reproduction && config.BindValues && valuesComplete {
			if snippet := reproductionSnippet(executor.driver, query, args); snippet != "" {
				attrs = append(attrs, String("reproduction", snippet))
			}
		}
	}

	executor.logger.Log(ctx, level, queryMessage, attrs...)
}

// explain captures a plan-only EXPLAIN on the observed executor, so the plan
// sees the same transaction and snapshot as the statement it describes.
func (executor *instrumentedExecutor) explain(ctx context.Context, query string, args []any) (string, error) {
	prefix, ok := explainPrefixes[executor.driver]
	if !ok {
		return "", nil
	}
	// A statement that was slow because its deadline expired cannot be explained
	// on the same context, and inventing a fresh one would run unbounded work.
	if err := ctx.Err(); err != nil {
		return "", err
	}
	rows, err := executor.inner.QueryContext(ctx, prefix+query, args...)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	return scanPlan(rows)
}

// explainPrefixes lists drivers with a known plan-only EXPLAIN form. ANALYZE is
// never used, because it would execute the observed statement a second time.
var explainPrefixes = map[string]string{
	"sqlite":     "EXPLAIN QUERY PLAN ",
	"sqlite3":    "EXPLAIN QUERY PLAN ",
	"postgres":   "EXPLAIN (FORMAT JSON) ",
	"postgresql": "EXPLAIN (FORMAT JSON) ",
	"pgx":        "EXPLAIN (FORMAT JSON) ",
	"mysql":      "EXPLAIN FORMAT=JSON ",
}

// SupportsExplain reports whether driver has a known plan-only EXPLAIN form.
// An unsupported driver keeps the query log and loses only the plan.
func SupportsExplain(driver string) bool {
	_, ok := explainPrefixes[driver]
	return ok
}

// scanPlan renders plan rows without interpreting them, because every dialect
// shapes its own columns.
func scanPlan(rows *sql.Rows) (string, error) {
	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}
	cells := make([]any, len(columns))
	holders := make([]any, len(columns))
	for i := range cells {
		holders[i] = &cells[i]
	}
	var plan strings.Builder
	for rows.Next() {
		if err := rows.Scan(holders...); err != nil {
			return "", err
		}
		if plan.Len() > 0 {
			plan.WriteByte('\n')
		}
		// A single-column plan is already prose or JSON, so naming the column
		// would only add noise. A wider one needs its column names to be read.
		first := true
		for i, cell := range cells {
			// SQLite names one of its plan columns "notused". Taking that at its
			// word is reading the dialect, not interpreting the plan.
			if columns[i] == "notused" {
				continue
			}
			if !first {
				plan.WriteByte(' ')
			}
			first = false
			if len(columns) > 1 {
				plan.WriteString(columns[i])
				plan.WriteByte('=')
			}
			plan.WriteString(planCell(cell))
		}
	}
	return plan.String(), rows.Err()
}

func planCell(cell any) string {
	switch typed := cell.(type) {
	case nil:
		return ""
	case []byte:
		return string(typed)
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

// renderArgs formats bind values for the record and reports whether every value
// survived intact, which is what decides if a rerun snippet is honest.
func renderArgs(args []any, config *QueryDiagnostics) ([]string, bool) {
	if !config.BindValues || len(args) == 0 {
		return nil, true
	}
	rendered := make([]string, len(args))
	complete := true
	for i, arg := range args {
		text, truncated := truncateText(displayValue(arg), config.MaxValueLength)
		rendered[i] = text
		if truncated {
			complete = false
		}
	}
	return rendered, complete
}

// displayValue renders one argument as a scalar. A non-scalar becomes a type
// marker rather than a dump of application state.
func displayValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "NULL"
	case string:
		return typed
	case []byte:
		return "0x" + hex.EncodeToString(typed)
	case bool:
		return strconv.FormatBool(typed)
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(typed)
	default:
		return fmt.Sprintf("<%T>", value)
	}
}

func truncateText(text string, limit int) (string, bool) {
	if limit <= 0 || len(text) <= limit {
		return text, false
	}
	return text[:limit], true
}
