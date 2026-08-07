package pwdata

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shibukawa/tinybind-go/sqlbind"
)

// pageSize bounds one read. A table viewer that fetched everything would hold
// the application's pool for as long as the largest table takes, which is the
// one thing a development tool must not do to the process it is observing.
const pageSize = 50

// valueLimit truncates a rendered cell. A blob column is otherwise a page that
// never finishes painting.
const valueLimit = 512

// Column is one column of one table, as the engine's own catalog describes it.
type Column struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	NotNull bool   `json:"notNull"`
	// PrimaryKey is the one-based position in the primary key, or zero when the
	// column is not part of one. Position matters because an edit addresses a
	// row by its whole key, in order.
	PrimaryKey int `json:"primaryKey"`
}

// Table is one table and what the pane knows about it before reading any row.
type Table struct {
	Name string `json:"name"`
	// Framework marks a table the framework owns rather than the application.
	// It is shown and readable — a developer looking at their own development
	// database is not the exposure policy:query-log-safety bounds — but it is
	// marked, because a row in one of these was written by code the developer
	// did not write and is not theirs to reason about.
	Framework bool `json:"framework"`
}

// Page is one bounded read of one table.
type Page struct {
	Table   string      `json:"table"`
	Columns []Column    `json:"columns"`
	Rows    [][]*string `json:"rows"`
	Offset  int         `json:"offset"`
	Limit   int         `json:"limit"`
	// Ordered reports whether the page has a stable order. Without a primary
	// key it does not, and saying so beats implying a stability the engine is
	// not promising.
	Ordered bool `json:"ordered"`
	More    bool `json:"more"`
}

// frameworkPrefix is the naming rule framework-owned tables follow, so an
// application reading its own schema can tell at a glance which it does not own.
const frameworkPrefix = "popcornwave_"

// Tables lists what the connected database holds.
func (c *Connection) Tables(ctx context.Context) ([]Table, error) {
	rows, err := c.queryRows(ctx, c.dialect.tables)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var tables []Table
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		// The migration bookkeeping is the engine's record of this tool's own
		// history and not application data, so it is listed like anything else
		// rather than hidden, but it is marked the same way.
		tables = append(tables, Table{
			Name:      name,
			Framework: strings.HasPrefix(name, frameworkPrefix) || name == "goose_db_version",
		})
	}
	return tables, rows.Err()
}

// Columns describes one table.
func (c *Connection) Columns(ctx context.Context, table string) ([]Column, error) {
	if err := c.knownTable(ctx, table); err != nil {
		return nil, err
	}
	rows, err := c.queryRows(ctx, c.dialect.columns, table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var columns []Column
	for rows.Next() {
		var column Column
		var notNull int
		if err := rows.Scan(&column.Name, &column.Type, &notNull, &column.PrimaryKey); err != nil {
			return nil, err
		}
		column.NotNull = notNull != 0
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

// Rows reads one page of one table.
//
// The table name is matched against the catalog before it reaches a statement,
// so what a request carries is a selection rather than SQL. Ordering is by
// primary key where there is one, because a page whose order changes between
// reads shows the same row twice and never shows another.
func (c *Connection) Rows(ctx context.Context, table string, offset int) (Page, error) {
	columns, err := c.Columns(ctx, table)
	if err != nil {
		return Page{}, err
	}
	if len(columns) == 0 {
		return Page{}, fmt.Errorf("table %q has no columns", table)
	}
	page := Page{Table: table, Columns: columns, Offset: offset, Limit: pageSize}
	statement := "SELECT " + c.columnList(columns) + " FROM " + c.dialect.quote(table)
	if order := c.primaryKeyOrder(columns); order != "" {
		statement += " ORDER BY " + order
		page.Ordered = true
	}
	// One extra row answers "is there another page" without a second count
	// query, which on a large table costs more than the page itself.
	statement += c.dialect.limitOffset(pageSize+1, offset)

	rows, err := c.queryRows(ctx, statement)
	if err != nil {
		return Page{}, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if len(page.Rows) == pageSize {
			page.More = true
			break
		}
		values := make([]any, len(columns))
		for index := range values {
			values[index] = new(any)
		}
		if err := rows.Scan(values...); err != nil {
			return Page{}, err
		}
		page.Rows = append(page.Rows, renderRow(values))
	}
	return page, rows.Err()
}

// scanRows reads every remaining row of a bounded result.
func scanRows(rows sqlbind.Rows, width int) ([][]*string, error) {
	var out [][]*string
	for rows.Next() {
		values := make([]any, width)
		for index := range values {
			values[index] = new(any)
		}
		if err := rows.Scan(values...); err != nil {
			return out, err
		}
		out = append(out, renderRow(values))
	}
	return out, rows.Err()
}

func errUnknownColumn(name string) error { return fmt.Errorf("no column named %q", name) }

// renderRow turns scanned values into displayable cells. A NULL stays nil, so
// the page can tell it apart from an empty string — which is the distinction a
// developer is usually looking at this table to check. The scan target is any
// rather than sql.RawBytes, because a native executor decodes into Go values
// and RawBytes only exists inside database/sql.
func renderRow(values []any) []*string {
	row := make([]*string, len(values))
	for index, value := range values {
		cell := *(value.(*any))
		if cell == nil {
			continue
		}
		text := renderCell(cell)
		if len(text) > valueLimit {
			text = text[:valueLimit] + "… (truncated)"
		}
		row[index] = &text
	}
	return row
}

func renderCell(cell any) string {
	switch typed := cell.(type) {
	case []byte:
		return string(typed)
	case string:
		return typed
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(typed)
	}
}

func (c *Connection) columnList(columns []Column) string {
	names := make([]string, len(columns))
	for index, column := range columns {
		names[index] = c.dialect.quote(column.Name)
	}
	return strings.Join(names, ", ")
}

func (c *Connection) primaryKeyOrder(columns []Column) string {
	keys := primaryKey(columns)
	if len(keys) == 0 {
		return ""
	}
	names := make([]string, len(keys))
	for index, column := range keys {
		names[index] = c.dialect.quote(column.Name)
	}
	return strings.Join(names, ", ")
}

// primaryKey returns the key columns in key order.
func primaryKey(columns []Column) []Column {
	var keys []Column
	for _, column := range columns {
		if column.PrimaryKey > 0 {
			keys = append(keys, column)
		}
	}
	for outer := 1; outer < len(keys); outer++ {
		for inner := outer; inner > 0 && keys[inner-1].PrimaryKey > keys[inner].PrimaryKey; inner-- {
			keys[inner-1], keys[inner] = keys[inner], keys[inner-1]
		}
	}
	return keys
}

// knownTable refuses a name the catalog does not report.
//
// This is the guard that keeps a request a selection. Every identifier this
// package puts into a statement has been through here first, so a name carrying
// SQL is rejected rather than quoted and hoped about.
func (c *Connection) knownTable(ctx context.Context, table string) error {
	tables, err := c.Tables(ctx)
	if err != nil {
		return err
	}
	for _, known := range tables {
		if known.Name == table {
			return nil
		}
	}
	return fmt.Errorf("no table named %q", table)
}

func knownColumn(columns []Column, name string) bool {
	for _, column := range columns {
		if column.Name == name {
			return true
		}
	}
	return false
}

// errNoPrimaryKey is returned when an edit cannot address a row.
var errNoPrimaryKey = errors.New("this table has no primary key, so a single row cannot be addressed; edit it with a statement instead")
