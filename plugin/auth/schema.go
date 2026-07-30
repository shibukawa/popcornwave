package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	authsqlite "github.com/shibukawa/popcornwave/contrib/authstate/sqlite"
	"github.com/shibukawa/popcornwave/plugin/session/rdb"
)

// MigrationName is the stable name of the migration a project carries for the
// tables this package owns, without a version. See rdb.MigrationName for why
// the version belongs to the project rather than to the package.
const MigrationName = "init_popcornwave_auth"

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

// CredentialTable holds the passkey credentials of the default
// CredentialStore. A deployment that installs its own store owns its own table
// and this one is neither created nor verified.
const CredentialTable = "popcornwave_passkey_credential"

// BootstrapTable holds the issued credentials that open a first passkey
// enrollment, for the default BootstrapStore.
const BootstrapTable = "popcornwave_auth_bootstrap"

// credentialSchemaSQL is the DDL of the passkey credential table.
//
// The key is stored twice on purpose: as the normalized COSE blob the ceremony
// produced, and as the curve points the relying party verifies with. It
// cross-checks the two, so a corrupted row fails closed instead of verifying
// against something else. The counter and backup state live beside them because
// an accepted assertion updates all of it in one statement.
const credentialSchemaSQL = `CREATE TABLE IF NOT EXISTS ` + CredentialTable + ` (
	credential_id BLOB PRIMARY KEY,
	account_id TEXT NOT NULL,
	user_handle BLOB NOT NULL,
	public_key BLOB NOT NULL,
	public_key_x BLOB NOT NULL,
	public_key_y BLOB NOT NULL,
	algorithm INTEGER NOT NULL,
	sign_count INTEGER NOT NULL DEFAULT 0,
	backup_eligible INTEGER NOT NULL DEFAULT 0,
	backup_state INTEGER NOT NULL DEFAULT 0,
	transports TEXT NOT NULL DEFAULT '',
	label TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL,
	last_used_at TIMESTAMP
)`

const credentialAccountIndexSQL = `CREATE INDEX IF NOT EXISTS ` + CredentialTable + `_account
	ON ` + CredentialTable + ` (account_id)`

// bootstrapSchemaSQL is the DDL of the bootstrap credential table.
//
// Only a digest of the secret is stored, and the attempt budget lives in the
// row so it can be decremented atomically rather than counted in memory.
const bootstrapSchemaSQL = `CREATE TABLE IF NOT EXISTS ` + BootstrapTable + ` (
	login_id TEXT PRIMARY KEY,
	account_id TEXT NOT NULL,
	secret_digest BLOB NOT NULL,
	purpose TEXT NOT NULL,
	issued_at TIMESTAMP NOT NULL,
	expires_at TIMESTAMP NOT NULL,
	attempts_remaining INTEGER NOT NULL,
	consumed_at TIMESTAMP
)`

// MigrationSQL returns the goose migration that creates the tables this package
// owns: the single-use OIDC correlation records, the pre-registration
// allowlist, and the default passkey credential and bootstrap stores.
func MigrationSQL() string {
	return `-- +goose Up
-- Owned by github.com/shibukawa/popcornwave/plugin/auth.
-- Single-use login correlation: state, nonce, and PKCE verifier of a pending
-- ceremony. Rows are consumed by the callback and swept after they expire.
` + authsqlite.SchemaSQL() + `;

-- Identities a deployment registered before their first login. Only consulted
-- when auth.oidc.admission is "registered".
` + allowlistSchemaSQL + `;

-- Passkey credentials of the default credential store. Unused when the
-- application installs its own store, and unused entirely in oidc_only.
` + credentialSchemaSQL + `;
` + credentialAccountIndexSQL + `;

-- Issued login IDs and secret digests that open one passkey enrollment.
` + bootstrapSchemaSQL + `;

-- +goose Down
DROP TABLE ` + BootstrapTable + `;
DROP TABLE ` + CredentialTable + `;
DROP TABLE ` + AllowlistTable + `;
DROP TABLE ` + authsqlite.TableName + `;
`
}

// requiredTables lists the framework tables and the migration that creates
// each one, in the order a project applies them.
//
// A table is required only when the selected mode reads it and the application
// installed no store of its own, so a deployment is never asked for a table
// nothing will ever write to.
func requiredTables(sessionTable string, config Config) [][2]string {
	required := [][2]string{
		{sessionTable, rdb.MigrationName},
		{authsqlite.TableName, MigrationName},
		{AllowlistTable, MigrationName},
	}
	if !config.usesPasskey() {
		return required
	}
	if installedCredentialStore() == nil {
		required = append(required, [2]string{CredentialTable, MigrationName})
	}
	if installedBootstrapStore() == nil && config.issuesBootstrapCredentials() {
		required = append(required, [2]string{BootstrapTable, MigrationName})
	}
	return required
}

// verifyTables reports a missing framework table together with the migration
// that creates it, so a deployment that skipped the migration fails at startup
// instead of during the first login.
//
// The migration is named without a version, because the version is whatever was
// free in that project when the file was written.
func verifyTables(ctx context.Context, db *sql.DB, sessionTable string, config Config) error {
	for _, required := range requiredTables(sessionTable, config) {
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
