// Package postgres registers the PostgreSQL engine for a postgres:// DSN.
//
// Blank-import it from the application entry point to link it:
//
//	import _ "github.com/shibukawa/popcornwave/database/postgres"
//
// A TinyGo build needs -scheduler=threads. Under the cooperative scheduler a
// blocking socket call holds the whole runtime, so the driver's cancellation
// watcher never runs and a query outlives its deadline without reporting one.
package postgres

import (
	"github.com/shibukawa/popcornwave/database"
	"github.com/shibukawa/tinygodriver/database/sql/pgxstdlib"
)

// Dialect is the canonical engine name this package registers.
const Dialect = "postgres"

func init() {
	database.Register(database.Engine{
		Dialect: Dialect,
		Schemes: []string{"postgres", "postgresql"},
		// The configured DSN is already a libpq URL, so it is handed over
		// whole rather than stripped of its scheme.
		KeepScheme: true,
		// pgxstdlib registers no database/sql driver name: it builds the pool
		// from a pgx connector, which is why the registry resolves schemes to
		// an opener instead of to a driver name.
		Open: pgxstdlib.Open,
	})
}
