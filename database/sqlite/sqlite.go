// Package sqlite registers the SQLite engine for a sqlite:// DSN.
//
// The framework already links it, so an application imports this package only
// to be explicit about what its DSN selects.
package sqlite

import (
	"database/sql"

	"github.com/shibukawa/popcornwave/database"
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
