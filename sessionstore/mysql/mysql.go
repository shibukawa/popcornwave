// Package mysql registers the MySQL dialect of the session store.
//
//	import _ "github.com/shibukawa/popcornwave/sessionstore/mysql"
//
// The import contributes the dialect and, through it, the rdb session backend.
// The database connection itself comes from popcornwave/database/mysql, which
// an application imports separately.
package mysql

import (
	"context"

	"github.com/shibukawa/popcornwave/sessionstore"
	"github.com/shibukawa/tinybind-go/sqlbind"
)

// Dialect is the registered engine name, which is also what a mysql:// DSN
// resolves to.
const Dialect = "mysql"

func init() {
	sessionstore.Register(sessionstore.Dialect{
		Name:        Dialect,
		CreateTable: createTable,
		Upsert:      upsert,
		Prune:       prune,
		Columns:     columns,
	})
}

// createTable gives the key an explicit length, because MySQL indexes no
// unbounded text, and the payload a MEDIUMBLOB, because a BLOB stops at 64 KiB
// while the store admits up to a megabyte.
func createTable(table string) string {
	return `CREATE TABLE IF NOT EXISTS ` + table + ` (
	key_hash VARCHAR(64) NOT NULL PRIMARY KEY,
	created_at_ms BIGINT NOT NULL,
	authenticated_at_ms BIGINT NOT NULL,
	last_seen_at_ms BIGINT NOT NULL,
	expires_at_ms BIGINT NOT NULL,
	idle_expires_at_ms BIGINT NOT NULL,
	method VARCHAR(64) NOT NULL,
	version INT NOT NULL,
	payload MEDIUMBLOB NOT NULL
)`
}

func upsert(table string) string {
	return `INSERT INTO ` + table + `(key_hash, created_at_ms, authenticated_at_ms, last_seen_at_ms,
			expires_at_ms, idle_expires_at_ms, method, version, payload)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			created_at_ms = VALUES(created_at_ms),
			authenticated_at_ms = VALUES(authenticated_at_ms),
			last_seen_at_ms = VALUES(last_seen_at_ms),
			expires_at_ms = VALUES(expires_at_ms),
			idle_expires_at_ms = VALUES(idle_expires_at_ms),
			method = VALUES(method),
			version = VALUES(version),
			payload = VALUES(payload)`
}

// prune deletes in place, because MySQL refuses a subquery that reads the
// table being deleted from but does accept ORDER BY and LIMIT here.
func prune(table string) string {
	return `DELETE FROM ` + table + `
		WHERE expires_at_ms <= ? OR (idle_expires_at_ms > 0 AND idle_expires_at_ms <= ?)
		ORDER BY expires_at_ms, key_hash LIMIT ?`
}

func columns(ctx context.Context, db sqlbind.SQLExecutor, table string) ([]string, error) {
	rows, err := sqlbind.Query(ctx, db, `
		SELECT column_name FROM information_schema.columns
		WHERE table_name = ? AND table_schema = database()
		ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return sessionstore.ScanColumns(rows)
}
