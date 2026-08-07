// Package pwdata serves the pw dev data pane from inside the application.
//
// It lives in the application process because that is the only place the
// development database can be reached. An embedded SQLite connection is a
// process-local handle rather than an endpoint, and an in-memory one has no
// external existence at all, so a tool outside the process can address neither.
// Running here also means every read and write goes through the pool, the
// driver, and the diagnostics the project actually configured.
//
// Nothing here reaches a release build. The framework starts it only under the
// pwdev build mode, and only when pw dev has told the process where its console
// is, so an application compiled by pw build links no call that starts it.
package pwdata

import (
	"context"
	"sort"
	"sync"

	"github.com/shibukawa/tinybind-go/sqlbind"
)

// Param is one parameter of a declared query, as its generated builder
// declares it.
type Param struct {
	Name string `json:"name"`
	// Kind is the Go type as written in the generated signature, which is what
	// the form has to be able to produce.
	Kind string `json:"kind"`
}

// Query is one declared statement, registered from inside the package the
// generator emitted it into.
//
// Registration happens there because a generated builder named for an
// unexported statement is unreachable from anywhere else, and the statements a
// developer most wants to try are not always the exported ones.
type Query struct {
	Package  string  `json:"package"`
	Name     string  `json:"name"`
	Exported bool    `json:"exported"`
	Params   []Param `json:"params"`
	// Build turns the supplied arguments into the statement the application
	// itself would run. It is the generated builder, so what the pane executes
	// is the project's own SQL rather than a second rendering of it.
	Build func(args []string) (sqlbind.Statement, error) `json:"-"`
}

var registry = struct {
	sync.RWMutex
	queries []Query
}{}

// RegisterQuery adds a declared statement. It is called from generated code at
// package initialisation, so it takes no error: a duplicate is a generation
// defect rather than something a running pane can report.
func RegisterQuery(query Query) {
	registry.Lock()
	defer registry.Unlock()
	registry.queries = append(registry.queries, query)
}

// Queries lists what was registered, ordered so the page is stable across runs.
func Queries() []Query {
	registry.RLock()
	defer registry.RUnlock()
	queries := make([]Query, len(registry.queries))
	copy(queries, registry.queries)
	sort.Slice(queries, func(i, j int) bool {
		if queries[i].Package != queries[j].Package {
			return queries[i].Package < queries[j].Package
		}
		return queries[i].Name < queries[j].Name
	})
	return queries
}

func lookupQuery(pkg, name string) (Query, bool) {
	for _, query := range Queries() {
		if query.Package == pkg && query.Name == name {
			return query, true
		}
	}
	return Query{}, false
}

// RunQuery executes one declared statement with the supplied arguments.
//
// The statement text comes from the generated builder, so the pane runs what
// the application would run rather than an imitation of it, and a query whose
// SQL is assembled conditionally is assembled the same way here.
func (c *Connection) RunQuery(ctx context.Context, pkg, name string, args []string) Result {
	query, ok := lookupQuery(pkg, name)
	if !ok {
		return Result{Error: "no declared query named " + pkg + "." + name}
	}
	statement, err := query.Build(args)
	if err != nil {
		return Result{Error: err.Error()}
	}
	result := Result{SQL: statement.SQL}
	// The same classification the statement console uses, for the same reason:
	// a declared write that ran through QueryContext would report no rows
	// instead of what it changed.
	if !returnsRows(statement.SQL) {
		return c.runWithoutRows(ctx, statement)
	}
	rows, err := c.queryRows(ctx, statement.SQL, statement.Args...)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = rows.Close() }()
	return readResult(result, rows)
}

func (c *Connection) runWithoutRows(ctx context.Context, statement sqlbind.Statement) Result {
	result := Result{SQL: statement.SQL}
	outcome, err := c.db.ExecContext(ctx, statement.SQL, statement.Args...)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if count, err := outcome.RowsAffected(); err == nil {
		result.Affected = count
	} else {
		result.Affected = -1
	}
	return result
}
