-- +goose Up
-- Owned by github.com/shibukawa/popcornwave/sessionstore.
-- Login sessions: one row per issued cookie token, keyed by its hash.
CREATE TABLE IF NOT EXISTS popcornwave_session (
	key_hash TEXT PRIMARY KEY,
	created_at_ms INTEGER NOT NULL,
	authenticated_at_ms INTEGER NOT NULL,
	last_seen_at_ms INTEGER NOT NULL,
	expires_at_ms INTEGER NOT NULL,
	idle_expires_at_ms INTEGER NOT NULL,
	method TEXT NOT NULL,
	version INTEGER NOT NULL,
	payload BLOB NOT NULL
);

-- +goose Down
DROP TABLE popcornwave_session;
