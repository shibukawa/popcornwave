-- +goose Up
CREATE TABLE accounts (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL,
	email TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE rooms (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	creator_account_id TEXT NOT NULL,
	title TEXT NOT NULL,
	subject TEXT NOT NULL,
	subject_description TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'open'
		CHECK (status IN ('open', 'closed')),
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	closed_at TIMESTAMP,
	FOREIGN KEY (creator_account_id) REFERENCES accounts (id),
	CHECK (
		(status = 'open' AND closed_at IS NULL)
		OR (status = 'closed' AND closed_at IS NOT NULL)
	)
);

CREATE INDEX rooms_created_at_desc
	ON rooms (created_at DESC, id DESC);

CREATE INDEX rooms_creator_created_at_desc
	ON rooms (creator_account_id, created_at DESC, id DESC);

CREATE TABLE room_participants (
	room_id INTEGER NOT NULL,
	account_id TEXT NOT NULL,
	joined_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (room_id, account_id),
	FOREIGN KEY (room_id) REFERENCES rooms (id) ON DELETE CASCADE,
	FOREIGN KEY (account_id) REFERENCES accounts (id) ON DELETE CASCADE
);

CREATE INDEX room_participants_account_joined_at_desc
	ON room_participants (account_id, joined_at DESC, room_id DESC);

-- +goose Down
DROP TABLE room_participants;
DROP TABLE rooms;
DROP TABLE accounts;
