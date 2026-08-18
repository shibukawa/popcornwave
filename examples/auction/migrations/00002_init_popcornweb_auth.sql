-- +goose Up
-- Owned by github.com/shibukawa/popcornweb/plugin/auth.
-- Single-use login correlation: state, nonce, and PKCE verifier of a pending
-- ceremony. Rows are consumed by the callback and swept after they expire.
CREATE TABLE IF NOT EXISTS popcornweb_authstate (
	namespace TEXT NOT NULL,
	"key" TEXT NOT NULL,
	expires_at_ms INTEGER NOT NULL,
	payload BLOB NOT NULL,
	PRIMARY KEY (namespace, "key")
) WITHOUT ROWID;

-- Identities a deployment registered before their first login. Only consulted
-- when auth.oidc.admission is "registered".
CREATE TABLE IF NOT EXISTS popcornweb_auth_allowlist (
	issuer TEXT NOT NULL,
	claim TEXT NOT NULL,
	value TEXT NOT NULL,
	note TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (issuer, claim, value)
);

-- Passkey credentials of the default credential store. Unused when the
-- application installs its own store, and unused entirely in oidc_only.
CREATE TABLE IF NOT EXISTS popcornweb_passkey_credential (
	credential_id BLOB NOT NULL PRIMARY KEY,
	account_id TEXT NOT NULL,
	user_handle BLOB NOT NULL,
	public_key BLOB NOT NULL,
	public_key_x BLOB NOT NULL,
	public_key_y BLOB NOT NULL,
	algorithm INTEGER NOT NULL,
	sign_count BIGINT NOT NULL DEFAULT 0,
	backup_eligible INTEGER NOT NULL DEFAULT 0,
	backup_state INTEGER NOT NULL DEFAULT 0,
	transports TEXT NOT NULL,
	label TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	last_used_at TIMESTAMP
);
CREATE INDEX IF NOT EXISTS popcornweb_passkey_credential_account
	ON popcornweb_passkey_credential (account_id);

-- Issued login IDs and secret digests that open one passkey enrollment.
CREATE TABLE IF NOT EXISTS popcornweb_auth_bootstrap (
	login_id TEXT NOT NULL PRIMARY KEY,
	account_id TEXT NOT NULL,
	secret_digest BLOB NOT NULL,
	purpose TEXT NOT NULL,
	issued_at TIMESTAMP NOT NULL,
	expires_at TIMESTAMP NOT NULL,
	attempts_remaining INTEGER NOT NULL,
	consumed_at TIMESTAMP
);

-- Tokens and identities withdrawn before their tokens expire. Only consulted
-- when auth.mode is "jwt_only" and auth.jwt.revocation.mode is not "off". A row
-- is a positive statement, so an unreachable table is an unknown rather than a
-- "not revoked".
CREATE TABLE IF NOT EXISTS popcornweb_auth_revocation (
	issuer TEXT NOT NULL,
	kind TEXT NOT NULL,
	key_value TEXT NOT NULL,
	revoked_at TIMESTAMP NOT NULL,
	expires_at TIMESTAMP NOT NULL,
	note TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (issuer, kind, key_value)
);
CREATE INDEX IF NOT EXISTS popcornweb_auth_revocation_expires
	ON popcornweb_auth_revocation (expires_at);

-- +goose Down
DROP TABLE popcornweb_auth_revocation;
DROP TABLE popcornweb_auth_bootstrap;
DROP TABLE popcornweb_passkey_credential;
DROP TABLE popcornweb_auth_allowlist;
DROP TABLE popcornweb_authstate;
