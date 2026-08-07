package pwdata

import "context"

// ForeignKey is one reference a table makes.
//
// It is what turns a grid of identifiers into something navigable: a column
// holding 42 means nothing until the row it points at is one click away, and
// that click is the difference between reading data and reading a table.
type ForeignKey struct {
	// Column is the column in this table.
	Column string `json:"column"`
	// Table and Target are what it references.
	Table  string `json:"table"`
	Target string `json:"target"`
}

// ForeignKeys reports what one table references.
//
// A dialect with no statement for it, or a database whose catalog refuses the
// question, yields none rather than an error: the grid is still readable
// without links, and losing the whole page over a missing affordance would be
// the wrong trade.
func (c *Connection) ForeignKeys(ctx context.Context, table string) map[string]ForeignKey {
	tables, err := c.Tables(ctx)
	if err != nil {
		return nil
	}
	return c.foreignKeys(ctx, tables, table)
}

// foreignKeys is ForeignKeys over an already-fetched catalog.
func (c *Connection) foreignKeys(ctx context.Context, tables []Table, table string) map[string]ForeignKey {
	if c.dialect.foreignKeys == "" {
		return nil
	}
	if err := knownTable(tables, table); err != nil {
		return nil
	}
	rows, err := c.queryRows(ctx, c.dialect.foreignKeys, table)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()
	keys := map[string]ForeignKey{}
	for rows.Next() {
		var key ForeignKey
		if err := rows.Scan(&key.Column, &key.Table, &key.Target); err != nil {
			return keys
		}
		keys[key.Column] = key
	}
	return keys
}

// Referenced reads the rows one foreign key value points at.
//
// The column and table come from the catalog, and the value travels as a bind
// parameter, so following a link is a selection rather than a query the page
// composed. That is what keeps this on the browsing side of the pane instead of
// being the filter box requirement:dev-data-pane declines to offer.
func (c *Connection) Referenced(ctx context.Context, table, column, value string) (Page, error) {
	tables, err := c.Tables(ctx)
	if err != nil {
		return Page{}, err
	}
	return c.referenced(ctx, tables, table, column, value)
}

// referenced is Referenced over an already-fetched catalog.
func (c *Connection) referenced(ctx context.Context, tables []Table, table, column, value string) (Page, error) {
	columns, err := c.columns(ctx, tables, table)
	if err != nil {
		return Page{}, err
	}
	if !knownColumn(columns, column) {
		return Page{}, errUnknownColumn(column)
	}
	page := Page{Table: table, Columns: columns, Limit: pageSize, Ordered: true}
	statement := "SELECT " + c.columnList(columns) + " FROM " + c.dialect.quote(table) +
		" WHERE " + c.dialect.quote(column) + " = " + c.dialect.placeholder(1) +
		c.dialect.limitOffset(pageSize, 0)
	rows, err := c.queryRows(ctx, statement, value)
	if err != nil {
		return Page{}, err
	}
	defer func() { _ = rows.Close() }()
	page.Rows, err = scanRows(rows, len(columns))
	return page, err
}
