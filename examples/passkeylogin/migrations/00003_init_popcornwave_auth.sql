-- +goose Up
-- Owned by github.com/shibukawa/popcornwave/plugin/auth.
-- Single-use login correlation: state, nonce, and PKCE verifier of a pending
-- ceremony. Rows are consumed by the callback and swept after they expire.
CREATE TABLE IF NOT EXISTS popcornwave_authstate (
	namespace TEXT NOT NULL,
	"key" TEXT NOT NULL,
	expires_at_ms INTEGER NOT NULL,
	payload BLOB NOT NULL,
	PRIMARY KEY (namespace, "key")
) WITHOUT ROWID;

-- Identities a deployment registered before their first login. Only consulted
-- when auth.oidc.admission is "registered".
CREATE TABLE IF NOT EXISTS popcornwave_auth_allowlist (
	issuer TEXT NOT NULL,
	claim TEXT NOT NULL,
	value TEXT NOT NULL,
	note TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (issuer, claim, value)
);

-- Passkey credentials of the default credential store. Unused when the
-- application installs its own store, and unused entirely in oidc_only.
CREATE TABLE IF NOT EXISTS popcornwave_passkey_credential (
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
);
CREATE INDEX IF NOT EXISTS popcornwave_passkey_credential_account
	ON popcornwave_passkey_credential (account_id);

-- Issued login IDs and secret digests that open one passkey enrollment.
CREATE TABLE IF NOT EXISTS popcornwave_auth_bootstrap (
	login_id TEXT PRIMARY KEY,
	account_id TEXT NOT NULL,
	secret_digest BLOB NOT NULL,
	purpose TEXT NOT NULL,
	issued_at TIMESTAMP NOT NULL,
	expires_at TIMESTAMP NOT NULL,
	attempts_remaining INTEGER NOT NULL,
	consumed_at TIMESTAMP
);

-- +goose Down
DROP TABLE popcornwave_auth_bootstrap;
DROP TABLE popcornwave_passkey_credential;
DROP TABLE popcornwave_auth_allowlist;
DROP TABLE popcornwave_authstate;
