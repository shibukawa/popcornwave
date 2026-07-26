package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	authsqlite "github.com/shibukawa/popcornwave/contrib/authstate/sqlite"
	"github.com/shibukawa/popcornwave/plugin/session/rdb"
)

// MigrationFileName is the migration a project carries for the tables this
// package owns. The leading version keeps framework tables ahead of application
// migrations.
const MigrationFileName = "00002_init_popcornwave_auth.sql"

// AllowlistTable holds the identities a deployment registered in advance. It is
// consulted by AdmissionRegistered.
const AllowlistTable = "popcornwave_auth_allowlist"

// allowlistSchemaSQL is the DDL of the pre-registration table.
//
// A row names one verified claim and its expected value, because a local
// deployment usually knows an operator's email or account name before that
// person has ever logged in and therefore before its subject exists.
const allowlistSchemaSQL = `CREATE TABLE IF NOT EXISTS ` + AllowlistTable + ` (
	issuer TEXT NOT NULL,
	claim TEXT NOT NULL,
	value TEXT NOT NULL,
	note TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (issuer, claim, value)
)`

// MigrationSQL returns the goose migration that creates the tables this package
// owns: the single-use OIDC correlation records and the pre-registration
// allowlist.
func MigrationSQL() string {
	return `-- +goose Up
-- Owned by github.com/shibukawa/popcornwave/plugin/auth.
-- Single-use login correlation: state, nonce, and PKCE verifier of a pending
-- ceremony. Rows are consumed by the callback and swept after they expire.
` + authsqlite.SchemaSQL() + `;

-- Identities a deployment registered before their first login. Only consulted
-- when auth.oidc.admission is "registered".
` + allowlistSchemaSQL + `;

-- +goose Down
DROP TABLE ` + AllowlistTable + `;
DROP TABLE ` + authsqlite.TableName + `;
`
}

// requiredTables lists the framework tables and the migration that creates
// each one, in the order a project applies them.
func requiredTables(sessionTable string) [][2]string {
	return [][2]string{
		{sessionTable, rdb.MigrationFileName},
		{authsqlite.TableName, MigrationFileName},
		{AllowlistTable, MigrationFileName},
	}
}

// verifyTables reports a missing framework table together with the migration
// that creates it, so a deployment that skipped the migration fails at startup
// instead of during the first login.
func verifyTables(ctx context.Context, db *sql.DB, sessionTable string) error {
	for _, required := range requiredTables(sessionTable) {
		exists, err := tableExists(ctx, db, required[0])
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("table %q is missing: apply migrations/%s with pw migrate up", required[0], required[1])
		}
	}
	return nil
}

func tableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var name string
	err := db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read schema: %w", err)
	}
	return true, nil
}
