// Package sqlite registers the SQLite dialect of the session store.
//
//	import _ "github.com/shibukawa/popcornwave/sessionstore/sqlite"
//
// The import contributes the dialect and, through it, the rdb session backend.
// It registers no database/sql driver: an engine package under
// popcornwave/database does that separately.
package sqlite

import (
	"context"
	"database/sql"

	"github.com/shibukawa/popcornwave/sessionstore"
)

// Dialect is the registered engine name, which is also what a sqlite:// DSN
// resolves to.
const Dialect = "sqlite"

func init() {
	sessionstore.Register(sessionstore.Dialect{
		Name:        Dialect,
		CreateTable: createTable,
		Upsert:      upsert,
		Prune:       prune,
		Columns:     columns,
	})
}

func createTable(table string) string {
	return `CREATE TABLE IF NOT EXISTS ` + table + ` (
	key_hash TEXT PRIMARY KEY,
	created_at_ms INTEGER NOT NULL,
	authenticated_at_ms INTEGER NOT NULL,
	last_seen_at_ms INTEGER NOT NULL,
	expires_at_ms INTEGER NOT NULL,
	idle_expires_at_ms INTEGER NOT NULL,
	method TEXT NOT NULL,
	version INTEGER NOT NULL,
	payload BLOB NOT NULL
)`
}

func upsert(table string) string {
	return `INSERT INTO ` + table + `(key_hash, created_at_ms, authenticated_at_ms, last_seen_at_ms,
			expires_at_ms, idle_expires_at_ms, method, version, payload)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(key_hash) DO UPDATE SET
			created_at_ms = excluded.created_at_ms,
			authenticated_at_ms = excluded.authenticated_at_ms,
			last_seen_at_ms = excluded.last_seen_at_ms,
			expires_at_ms = excluded.expires_at_ms,
			idle_expires_at_ms = excluded.idle_expires_at_ms,
			method = excluded.method,
			version = excluded.version,
			payload = excluded.payload`
}

// prune deletes through a subquery, which is how SQLite bounds a delete
// without the optional LIMIT support its build may lack.
func prune(table string) string {
	return `DELETE FROM ` + table + ` WHERE key_hash IN (
			SELECT key_hash FROM ` + table + `
			WHERE expires_at_ms <= ? OR (idle_expires_at_ms > 0 AND idle_expires_at_ms <= ?)
			ORDER BY expires_at_ms, key_hash LIMIT ?
		)`
}

// columns reads the schema through PRAGMA, because SQLite carries no
// information_schema. A missing table answers with no rows rather than an
// error, which is what VerifySchema reads as "not migrated yet".
func columns(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}
