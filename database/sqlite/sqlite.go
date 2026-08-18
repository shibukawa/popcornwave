// Package sqlite registers the SQLite engine for a sqlite:// DSN.
//
// Applications link it explicitly, normally through the blank import generated
// by pw init or pw add database. A binary that uses no SQLite carries no driver.
package sqlite

import (
	"database/sql"

	"github.com/shibukawa/popcornweb/database"
	tinygosqlite "github.com/shibukawa/tinygodriver/database/sql/sqlite"
)

// Dialect is the canonical engine name this package registers.
const Dialect = "sqlite"

func init() {
	database.Register(database.Engine{
		Dialect: Dialect,
		// sqlite3 is accepted because goose and much existing configuration
		// spell it that way; both resolve to the one dialect above.
		Schemes: []string{"sqlite", "sqlite3"},
		// The rest of the DSN is a path or :memory:, never a URL, so the
		// scheme is dropped before the driver sees it.
		Open: func(dataSource string) (*sql.DB, error) {
			return sql.Open(tinygosqlite.DriverName, dataSource)
		},
	})
}
