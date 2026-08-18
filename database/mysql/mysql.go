// Package mysql registers the MySQL and MariaDB engine for a mysql:// DSN.
//
// Blank-import it from the application entry point to link it:
//
//	import _ "github.com/shibukawa/popcornweb/database/mysql"
//
// A TinyGo build needs -scheduler=threads, for the reason recorded in the
// postgres package. The TinyGo backend of the underlying driver is MPL-2.0
// while the rest of it is Apache-2.0, so a distributed binary that links this
// package carries both notices.
package mysql

import (
	"database/sql"
	"strings"

	"github.com/shibukawa/popcornweb/database"
	tinygomysql "github.com/shibukawa/tinygodriver/database/sql/mysql"
)

// Dialect is the canonical engine name this package registers.
const Dialect = "mysql"

func init() {
	database.Register(database.Engine{
		Dialect: Dialect,
		Schemes: []string{"mysql"},
		// A go-sql-driver DSN is user:pass@tcp(host:port)/db, which is not a
		// URL, so the scheme prefix is dropped before the driver sees it.
		Open: open,
	})
}

func open(dataSource string) (*sql.DB, error) {
	return tinygomysql.Open(withParseTime(dataSource))
}

// withParseTime defaults parseTime=true, which MySQL needs before a DATETIME
// or TIMESTAMP column will scan into a time.Time. Without it the driver hands
// back []byte and every such Scan fails, which is how the migration engine's
// own applied-at column broke: the migration applied and reading its state
// afterwards did not.
//
// An explicit setting is left alone, including parseTime=false, because an
// operator who wrote it meant it.
func withParseTime(dataSource string) string {
	// Parameters begin at the first ? after the last /, which is where the
	// database name ends. Looking from the last / keeps a password or an
	// address containing ? out of the search.
	path := strings.LastIndex(dataSource, "/")
	if path < 0 {
		// Not a DSN this function can reason about; the driver reports it.
		return dataSource
	}
	query := strings.Index(dataSource[path:], "?")
	if query < 0 {
		return dataSource + "?parseTime=true"
	}
	for _, parameter := range strings.Split(dataSource[path+query+1:], "&") {
		if name, _, found := strings.Cut(parameter, "="); found && name == "parseTime" {
			return dataSource
		}
	}
	return dataSource + "&parseTime=true"
}
