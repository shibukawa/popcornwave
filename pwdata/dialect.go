package pwdata

import (
	"strconv"
	"strings"
)

// dialect is everything the data pane has to spell differently per engine.
//
// Three engines are first class, and they disagree about all three things this
// pane needs: how to quote an identifier, how to write a placeholder, and where
// the catalog lives. Keeping the differences in one table means adding an
// engine is one entry rather than a branch in every query below.
type dialect struct {
	name string
	// quote wraps an identifier. Every identifier this package emits comes
	// from the catalog rather than from a request, but it is still quoted:
	// a table legitimately named `order` or `select` is otherwise a syntax
	// error rather than a table.
	quote func(string) string
	// placeholder writes the nth bind marker, one-based.
	placeholder func(int) string
	// tables lists the application's own tables.
	tables string
	// columns describes one table: name, type, nullability, primary key
	// position. The statement takes the table name as its only argument.
	columns string
	// limitOffset writes the paging clause.
	limitOffset func(limit, offset int) string
}

func dialectFor(driver string) dialect {
	switch normalizeDriver(driver) {
	case "postgres":
		return postgresDialect
	case "mysql":
		return mysqlDialect
	default:
		return sqliteDialect
	}
}

// normalizeDriver folds the spellings one engine answers to. The DSN scheme,
// the registered driver name, and the goose dialect do not always agree, and
// this pane only has to know which of the three engines it is talking to.
func normalizeDriver(driver string) string {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "postgres", "postgresql", "pgx":
		return "postgres"
	case "mysql":
		return "mysql"
	default:
		return "sqlite"
	}
}

func doubleQuote(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func backQuote(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func questionMark(int) string { return "?" }

func dollar(index int) string { return "$" + strconv.Itoa(index) }

func plainLimit(limit, offset int) string {
	return " LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset)
}

// SQLite keeps its catalog in sqlite_master and describes a table through a
// pragma, which is why it cannot share the information_schema statements the
// other two use. The pragma is exposed as a table-valued function, which is
// what lets it be selected from with a bind parameter rather than assembled by
// hand.
var sqliteDialect = dialect{
	name:        "sqlite",
	quote:       doubleQuote,
	placeholder: questionMark,
	tables: `SELECT name FROM sqlite_master
		WHERE type = 'table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name`,
	columns: `SELECT name, type, "notnull", pk
		FROM pragma_table_info(?)
		ORDER BY cid`,
	limitOffset: plainLimit,
}

var postgresDialect = dialect{
	name:        "postgres",
	quote:       doubleQuote,
	placeholder: dollar,
	tables: `SELECT table_name FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'
		ORDER BY table_name`,
	// The primary key position comes from the constraint rather than from the
	// column, because information_schema has no per-column key flag.
	columns: `SELECT c.column_name, c.data_type,
			CASE WHEN c.is_nullable = 'NO' THEN 1 ELSE 0 END,
			COALESCE(k.ordinal_position, 0)
		FROM information_schema.columns c
		LEFT JOIN information_schema.table_constraints t
			ON t.table_schema = c.table_schema AND t.table_name = c.table_name
			AND t.constraint_type = 'PRIMARY KEY'
		LEFT JOIN information_schema.key_column_usage k
			ON k.constraint_name = t.constraint_name AND k.table_schema = t.table_schema
			AND k.column_name = c.column_name
		WHERE c.table_schema = current_schema() AND c.table_name = $1
		ORDER BY c.ordinal_position`,
	limitOffset: plainLimit,
}

var mysqlDialect = dialect{
	name:        "mysql",
	quote:       backQuote,
	placeholder: questionMark,
	tables: `SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
		ORDER BY table_name`,
	// MySQL does carry a per-column key flag, so the constraint join the other
	// server engine needs is unnecessary here.
	columns: `SELECT column_name, column_type,
			CASE WHEN is_nullable = 'NO' THEN 1 ELSE 0 END,
			CASE WHEN column_key = 'PRI' THEN ordinal_position ELSE 0 END
		FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ?
		ORDER BY ordinal_position`,
	limitOffset: plainLimit,
}
