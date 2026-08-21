package pwdata

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shibukawa/tinybind-go/sqlbind"
)

// RowEdit addresses one row and says what to do with it.
//
// The key is the whole primary key, because that is what addresses exactly one
// row; a table without one cannot be edited here and says so. Values are bind
// parameters throughout — only column names reach the statement text, and only
// after the catalog has confirmed them.
type RowEdit struct {
	Table string `json:"table"`
	// Key maps primary key column names to their current values. It is empty
	// for an insert.
	Key map[string]string `json:"key"`
	// Values maps column names to their new values. A column absent here is
	// left alone on an update.
	Values map[string]string `json:"values"`
	// Nulls names columns to set to NULL, which an empty string cannot express.
	Nulls []string `json:"nulls"`
}

// UpdateRow writes one row addressed by its primary key.
func (c *Connection) UpdateRow(ctx context.Context, edit RowEdit) (int64, error) {
	columns, err := c.Columns(ctx, edit.Table)
	if err != nil {
		return 0, err
	}
	keys := primaryKey(columns)
	if len(keys) == 0 {
		return 0, errNoPrimaryKey
	}
	assignments, arguments, err := c.assignments(columns, edit)
	if err != nil {
		return 0, err
	}
	if len(assignments) == 0 {
		return 0, fmt.Errorf("nothing to change")
	}
	where, keyArguments, err := c.keyPredicate(keys, edit.Key, len(arguments))
	if err != nil {
		return 0, err
	}
	statement := "UPDATE " + c.dialect.quote(edit.Table) +
		" SET " + strings.Join(assignments, ", ") + " WHERE " + where
	return c.affected(ctx, statement, append(arguments, keyArguments...))
}

// InsertRow adds one row. Values not named are left to the column default,
// which is the difference between an insert here and one that would overwrite
// what the schema decided.
func (c *Connection) InsertRow(ctx context.Context, edit RowEdit) (int64, error) {
	columns, err := c.Columns(ctx, edit.Table)
	if err != nil {
		return 0, err
	}
	if err := knownColumns(columns, edit); err != nil {
		return 0, err
	}
	var names []string
	var markers []string
	var arguments []any
	for _, column := range columns {
		value, ok := edit.Values[column.Name]
		null := contains(edit.Nulls, column.Name)
		if !ok && !null {
			continue
		}
		names = append(names, c.dialect.quote(column.Name))
		markers = append(markers, c.dialect.placeholder(len(arguments)+1))
		arguments = append(arguments, nullable(value, null))
	}
	if len(names) == 0 {
		return 0, fmt.Errorf("no values to insert")
	}
	statement := "INSERT INTO " + c.dialect.quote(edit.Table) +
		" (" + strings.Join(names, ", ") + ") VALUES (" + strings.Join(markers, ", ") + ")"
	return c.affected(ctx, statement, arguments)
}

// DeleteRow removes one row addressed by its primary key.
func (c *Connection) DeleteRow(ctx context.Context, edit RowEdit) (int64, error) {
	columns, err := c.Columns(ctx, edit.Table)
	if err != nil {
		return 0, err
	}
	keys := primaryKey(columns)
	if len(keys) == 0 {
		return 0, errNoPrimaryKey
	}
	where, arguments, err := c.keyPredicate(keys, edit.Key, 0)
	if err != nil {
		return 0, err
	}
	return c.affected(ctx, "DELETE FROM "+c.dialect.quote(edit.Table)+" WHERE "+where, arguments)
}

func (c *Connection) assignments(columns []Column, edit RowEdit) ([]string, []any, error) {
	var assignments []string
	var arguments []any
	for _, column := range columns {
		value, ok := edit.Values[column.Name]
		null := contains(edit.Nulls, column.Name)
		if !ok && !null {
			continue
		}
		assignments = append(assignments,
			c.dialect.quote(column.Name)+" = "+c.dialect.placeholder(len(arguments)+1))
		arguments = append(arguments, nullable(value, null))
	}
	if err := knownColumns(columns, edit); err != nil {
		return nil, nil, err
	}
	return assignments, arguments, nil
}

// knownColumns refuses an edit naming a column the table does not have. The
// builders range over the catalog, so an unknown name could never reach the
// statement — but it would be silently discarded, and silently doing nothing is
// the worst answer a data editor can give.
func knownColumns(columns []Column, edit RowEdit) error {
	for name := range edit.Values {
		if !knownColumn(columns, name) {
			return fmt.Errorf("no column named %q", name)
		}
	}
	for _, name := range edit.Nulls {
		if !knownColumn(columns, name) {
			return fmt.Errorf("no column named %q", name)
		}
	}
	return nil
}

// keyPredicate builds the WHERE clause addressing one row. Offset continues the
// placeholder numbering the SET clause started, which only PostgreSQL cares
// about but every dialect is given consistently.
func (c *Connection) keyPredicate(keys []Column, values map[string]string, offset int) (string, []any, error) {
	var predicates []string
	var arguments []any
	for _, column := range keys {
		value, ok := values[column.Name]
		if !ok {
			return "", nil, fmt.Errorf("the key column %q was not supplied", column.Name)
		}
		predicates = append(predicates,
			c.dialect.quote(column.Name)+" = "+c.dialect.placeholder(offset+len(arguments)+1))
		arguments = append(arguments, value)
	}
	return strings.Join(predicates, " AND "), arguments, nil
}

// affected runs a write and reports the rows it changed.
//
// An edit that matched no row is reported rather than treated as success: it
// means the row moved or was already gone, and silently doing nothing is the
// worst answer a data editor can give.
func (c *Connection) affected(ctx context.Context, statement string, arguments []any) (int64, error) {
	result, err := c.db.ExecContext(ctx, statement, arguments...)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		// Not every driver reports it. Saying so is better than reporting a
		// zero that reads as "nothing matched".
		return -1, nil
	}
	return count, nil
}

func nullable(value string, null bool) any {
	if null {
		return nil
	}
	return value
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// Result is what a statement produced, whether it returned rows or a count.
type Result struct {
	Columns []string    `json:"columns"`
	Rows    [][]*string `json:"rows"`
	// Affected is set for a statement that changed rows rather than returning
	// them. Negative means the driver would not say.
	Affected int64  `json:"affected"`
	Returned bool   `json:"returned"`
	SQL      string `json:"sql"`
	Error    string `json:"error,omitempty"`
	// Truncated reports that the result had more rows than the pane will show.
	Truncated bool `json:"truncated"`
}

// Exec runs a statement the developer wrote and returns whatever it produced.
//
// This is a development database reached from a loopback console in a build
// mode api:cli-build cannot emit, and a developer who can edit rows can already
// write any statement through the schema. Bounding what they may type would
// therefore buy nothing; what is bounded is the number of rows that come back.
func (c *Connection) Exec(ctx context.Context, statement string) Result {
	result := Result{SQL: statement}
	trimmed := strings.TrimSpace(statement)
	if trimmed == "" {
		result.Error = "nothing to run"
		return result
	}
	if !returnsRows(trimmed) {
		return c.execWithoutRows(ctx, trimmed)
	}
	rows, err := c.queryRows(ctx, trimmed)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = rows.Close() }()
	return readResult(result, rows)
}

// returnsRows decides which of the two database/sql calls a statement wants,
// from its text rather than by trying one and falling back.
//
// Trying is what the first version did, and it was wrong in a way only a write
// reveals: SQLite answers QueryContext for an UPDATE without complaint, so the
// statement ran, reported no rows, and never reported what it changed. Falling
// back to ExecContext after that would have run it a second time.
func returnsRows(statement string) bool {
	word := strings.ToUpper(firstWord(statement))
	switch word {
	case "SELECT", "WITH", "VALUES", "TABLE", "SHOW", "EXPLAIN", "PRAGMA", "DESCRIBE", "DESC":
		return true
	}
	// A write with RETURNING is the shape flow:sql-generation emits most often,
	// and it is the one write whose rows are the point of running it.
	return hasReturning(statement)
}

func firstWord(statement string) string {
	fields := strings.Fields(statement)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimLeft(fields[0], "(")
}

// hasReturning looks for the keyword as a word, so a column or table named
// "returning_date" does not change how its statement is run.
func hasReturning(statement string) bool {
	upper := strings.ToUpper(statement)
	for index := 0; ; {
		found := strings.Index(upper[index:], "RETURNING")
		if found < 0 {
			return false
		}
		found += index
		before := found == 0 || !isWordByte(upper[found-1])
		after := found+len("RETURNING") >= len(upper) || !isWordByte(upper[found+len("RETURNING")])
		if before && after {
			return true
		}
		index = found + len("RETURNING")
	}
}

func isWordByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func (c *Connection) execWithoutRows(ctx context.Context, statement string) Result {
	result := Result{SQL: statement}
	outcome, err := c.db.ExecContext(ctx, statement)
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

// resultLimit bounds what one statement may paint. A developer who wants more
// adds their own LIMIT, which is a thing they can already express.
const resultLimit = 200

func readResult(result Result, rows sqlbind.Rows) Result {
	names, err := rows.Columns()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Columns, result.Returned = names, true
	for rows.Next() {
		if len(result.Rows) == resultLimit {
			result.Truncated = true
			break
		}
		values := make([]any, len(names))
		for index := range values {
			values[index] = new(any)
		}
		if err := rows.Scan(values...); err != nil {
			result.Error = err.Error()
			return result
		}
		result.Rows = append(result.Rows, renderRow(values))
	}
	if err := rows.Err(); err != nil {
		result.Error = err.Error()
	}
	return result
}

// errReadOnlyConnection is what a write attempt through a replica reports.
var errReadOnlyConnection = errors.New(
	"this connection is a read-only replica, so it cannot be written through; " +
		"select a writable connection to edit")
