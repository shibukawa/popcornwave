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

-- +goose Down
DROP TABLE popcornwave_auth_allowlist;
DROP TABLE popcornwave_authstate;
