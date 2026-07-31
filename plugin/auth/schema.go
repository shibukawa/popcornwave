package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/shibukawa/popcornwave/authstate"
)

// MigrationName is the stable name of the migration a project carries for the
// tables this package owns, without a version. See rdb.MigrationName for why
// the version belongs to the project rather than to the package.
const MigrationName = "init_popcornwave_auth"

// AllowlistTable holds the identities a deployment registered in advance. It is
// consulted by AdmissionRegistered.
const AllowlistTable = "popcornwave_auth_allowlist"

// allowlistSchemaSQL is the DDL of the pre-registration table under one
// engine.
//
// A row names one verified claim and its expected value, because a local
// deployment usually knows an operator's email or account name before that
// person has ever logged in and therefore before its subject exists. MySQL
// indexes no unbounded text, so its three key columns carry a length.
func allowlistSchemaSQL(dialect string) string {
	text, key := "TEXT", "TEXT"
	if dialect == "mysql" {
		key = "VARCHAR(255)"
	}
	return `CREATE TABLE IF NOT EXISTS ` + AllowlistTable + ` (
	issuer ` + key + ` NOT NULL,
	claim ` + key + ` NOT NULL,
	value ` + key + ` NOT NULL,
	note ` + text + ` NOT NULL DEFAULT '',
	PRIMARY KEY (issuer, claim, value)
)`
}

// MigrationSQL returns the goose migration that creates the tables this
// package owns under one engine: the single-use OIDC correlation records and
// the pre-registration allowlist.
func MigrationSQL(dialect string) (string, error) {
	stateSchema, err := authstate.SchemaSQL(dialect)
	if err != nil {
		return "", err
	}
	return `-- +goose Up
-- Owned by github.com/shibukawa/popcornwave/plugin/auth.
-- Single-use login correlation: state, nonce, and PKCE verifier of a pending
-- ceremony. Rows are consumed by the callback and swept after they expire.
` + stateSchema + `;

-- Identities a deployment registered before their first login. Only consulted
-- when auth.oidc.admission is "registered".
` + allowlistSchemaSQL(dialect) + `;

-- +goose Down
DROP TABLE ` + AllowlistTable + `;
DROP TABLE ` + authstate.TableName + `;
`, nil
}

// requiredTables lists the tables this package owns and the migration that
// creates each one. The session table is not among them: a session backend
// verifies its own storage, and a cookie or Redis backend has no table here at
// all.
func requiredTables() [][2]string {
	return [][2]string{
		{authstate.TableName, MigrationName},
		{AllowlistTable, MigrationName},
	}
}

// verifyTables reports a missing framework table together with the migration
// that creates it, so a deployment that skipped the migration fails at startup
// instead of during the first login.
//
// The migration is named without a version, because the version is whatever was
// free in that project when the file was written.
func verifyTables(ctx context.Context, db *sql.DB) error {
	for _, required := range requiredTables() {
		exists, err := tableExists(ctx, db, required[0])
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("table %q is missing: apply the migration named %s with pw migrate up",
				required[0], required[1])
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
