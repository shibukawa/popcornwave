// Package postgres registers the PostgreSQL dialect of the session store.
//
//	import _ "github.com/shibukawa/popcornweb/sessionstore/postgres"
//
// The import contributes the dialect and, through it, the rdb session backend.
// The database connection itself comes from popcornweb/database/postgres,
// which an application imports separately.
package postgres

import (
	"context"

	"github.com/shibukawa/popcornweb/sessionstore"
	"github.com/shibukawa/tinybind-go/sqlbind"
)

// Dialect is the registered engine name, which is also what a postgres:// or
// postgresql:// DSN resolves to.
const Dialect = "postgres"

func init() {
	sessionstore.Register(sessionstore.Dialect{
		Name:        Dialect,
		CreateTable: createTable,
		Upsert:      upsert,
		Prune:       prune,
		Columns:     columns,
		Rebind:      sessionstore.NumberedPlaceholders,
	})
}

func createTable(table string) string {
	return `CREATE TABLE IF NOT EXISTS ` + table + ` (
	key_hash TEXT PRIMARY KEY,
	created_at_ms BIGINT NOT NULL,
	authenticated_at_ms BIGINT NOT NULL,
	last_seen_at_ms BIGINT NOT NULL,
	expires_at_ms BIGINT NOT NULL,
	idle_expires_at_ms BIGINT NOT NULL,
	method TEXT NOT NULL,
	version INTEGER NOT NULL,
	payload BYTEA NOT NULL
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

// prune bounds the delete through a subquery, because PostgreSQL accepts no
// LIMIT on DELETE itself.
func prune(table string) string {
	return `DELETE FROM ` + table + ` WHERE key_hash IN (
			SELECT key_hash FROM ` + table + `
			WHERE expires_at_ms <= ? OR (idle_expires_at_ms > 0 AND idle_expires_at_ms <= ?)
			ORDER BY expires_at_ms, key_hash LIMIT ?
		)`
}

func columns(ctx context.Context, db sqlbind.SQLExecutor, table string) ([]string, error) {
	rows, err := sqlbind.Query(ctx, db, `
		SELECT column_name FROM information_schema.columns
		WHERE table_name = $1 AND table_schema = current_schema()
		ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return sessionstore.ScanColumns(rows)
}
