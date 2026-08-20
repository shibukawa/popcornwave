package pwdata

import (
	"context"
	"strings"
)

// Explain runs the plan-only form of a statement.
//
// The prefixes are the ones requirement:query-diagnostics already uses for a
// slow statement, so a plan read here and a plan attached to a data:query-record
// come from the same request. ANALYZE is never used: it would execute the
// statement a second time, which is the wrong thing to do to a write and a
// waste on a read.
func (c *Connection) Explain(ctx context.Context, statement string) Result {
	trimmed := strings.TrimSpace(statement)
	if trimmed == "" {
		return Result{Error: "nothing to explain"}
	}
	prefix, ok := explainPrefix(c.dialect.name)
	if !ok {
		return Result{Error: "this engine has no plan-only EXPLAIN form, so a plan cannot be read here"}
	}
	return c.collect(ctx, prefix+trimmed)
}

// ExplainQuery reads the plan of a declared statement, built with the arguments
// a run would use, so the plan is of the statement the application would run.
func (c *Connection) ExplainQuery(ctx context.Context, pkg, name string, args []string) Result {
	query, ok := lookupQuery(pkg, name)
	if !ok {
		return Result{Error: "no declared query named " + pkg + "." + name}
	}
	statement, err := query.Build(args)
	if err != nil {
		return Result{Error: err.Error()}
	}
	prefix, ok := explainPrefix(c.dialect.name)
	if !ok {
		return Result{Error: "this engine has no plan-only EXPLAIN form, so a plan cannot be read here"}
	}
	return c.collect(ctx, prefix+statement.SQL, statement.Args...)
}

// collect runs a statement that returns rows and collects them.
func (c *Connection) collect(ctx context.Context, statement string, args ...any) Result {
	result := Result{SQL: statement}
	rows, err := c.queryRows(ctx, statement, args...)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = rows.Close() }()
	return readResult(result, rows)
}

// explainPrefixes mirrors the per-dialect forms rule:explain-dialect-support
// names. They are spelled here rather than imported because the runtime keys
// them by driver name and this package has already folded those to an engine.
var explainPrefixes = map[string]string{
	"sqlite":   "EXPLAIN QUERY PLAN ",
	"postgres": "EXPLAIN (FORMAT JSON) ",
	"mysql":    "EXPLAIN FORMAT=JSON ",
}

func explainPrefix(engine string) (string, bool) {
	prefix, ok := explainPrefixes[engine]
	return prefix, ok
}
